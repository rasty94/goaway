package server

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	model "goaway/backend/dns/server/models"
	"io"
	"net"
	"sync"
	"time"

	"codeberg.org/miekg/dns"
	"github.com/quic-go/quic-go"
)

const (
	doqALPN        = "doq"
	doqDefaultPort = "853"
	doqMaxMsgSize  = 65535
)

// DoQServer serves DNS-over-QUIC (RFC 9250) requests using a QUIC listener.
type DoQServer struct {
	addr      string
	tlsConfig *tls.Config
	handler   dns.Handler
	listener  *quic.Listener
	mu        sync.Mutex
	closed    bool
}

// InitDoQ creates a configured DoQ server bound to dns.address:dns.ports.doq.
func (s *DNSServer) InitDoQ(cert tls.Certificate) (*DoQServer, error) {
	return s.InitDoQWithAddr(fmt.Sprintf("%s:%d", s.Config.DNS.Address, s.Config.DNS.Ports.DoQ), cert, s)
}

// InitDoQWithAddr creates a DoQ server on an explicit address and handler (also used in tests).
func (s *DNSServer) InitDoQWithAddr(addr string, cert tls.Certificate, handler dns.Handler) (*DoQServer, error) {
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{doqALPN},
		MinVersion:   tls.VersionTLS13,
	}

	listener, err := quic.ListenAddr(addr, tlsConfig, &quic.Config{
		MaxIdleTimeout:  30 * time.Second,
		KeepAlivePeriod: 15 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to listen DoQ on %s: %w", addr, err)
	}

	return &DoQServer{
		addr:      listener.Addr().String(),
		tlsConfig: tlsConfig,
		handler:   handler,
		listener:  listener,
	}, nil
}

// Addr returns the bound network address of the DoQ listener.
func (d *DoQServer) Addr() string {
	return d.addr
}

// ListenAndServe accepts QUIC connections and dispatches RFC 9250 streams to the DNS handler.
func (d *DoQServer) ListenAndServe() error {
	log.Info("Started DoQ (dns-over-quic) server on %s", d.addr)
	for {
		conn, err := d.listener.Accept(context.Background())
		if err != nil {
			d.mu.Lock()
			closed := d.closed
			d.mu.Unlock()
			if closed {
				return nil
			}
			return err
		}
		go d.serveConn(conn)
	}
}

// Shutdown closes the underlying QUIC listener.
func (d *DoQServer) Shutdown(_ context.Context) error {
	d.mu.Lock()
	d.closed = true
	d.mu.Unlock()
	if d.listener != nil {
		return d.listener.Close()
	}
	return nil
}

func (d *DoQServer) serveConn(conn *quic.Conn) {
	for {
		stream, err := conn.AcceptStream(conn.Context())
		if err != nil {
			return
		}
		go d.serveStream(conn, stream)
	}
}

func (d *DoQServer) serveStream(conn *quic.Conn, stream *quic.Stream) {
	defer func() { _ = stream.Close() }()

	_ = stream.SetDeadline(time.Now().Add(10 * time.Second))

	var lenBuf [2]byte
	if _, err := io.ReadFull(stream, lenBuf[:]); err != nil {
		return
	}
	msgLen := int(binary.BigEndian.Uint16(lenBuf[:]))
	if msgLen == 0 {
		return
	}

	raw := make([]byte, msgLen)
	if _, err := io.ReadFull(stream, raw); err != nil {
		return
	}

	msg := new(dns.Msg)
	msg.Data = raw
	if err := msg.Unpack(); err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(conn.Context(), 10*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, model.DoQ, true)

	rw := &doqResponseWriter{
		stream: stream,
		local:  conn.LocalAddr(),
		remote: conn.RemoteAddr(),
	}
	d.handler.ServeDNS(ctx, rw, msg)
}

type doqResponseWriter struct {
	stream *quic.Stream
	local  net.Addr
	remote net.Addr
}

func (w *doqResponseWriter) Write(b []byte) (int, error) {
	if len(b) > doqMaxMsgSize {
		return 0, fmt.Errorf("DoQ response too large: %d bytes", len(b))
	}
	var lenBuf [2]byte
	// #nosec G115 - length is explicitly bounded to doqMaxMsgSize (65535)
	binary.BigEndian.PutUint16(lenBuf[:], uint16(len(b)))
	if _, err := w.stream.Write(lenBuf[:]); err != nil {
		return 0, err
	}
	return w.stream.Write(b)
}

func (w *doqResponseWriter) LocalAddr() net.Addr   { return w.local }
func (w *doqResponseWriter) RemoteAddr() net.Addr  { return w.remote }
func (w *doqResponseWriter) Conn() net.Conn        { return nil }
func (w *doqResponseWriter) Session() *dns.Session { return nil }
func (w *doqResponseWriter) Hijack()        {}
func (w *doqResponseWriter) Close() error   { return w.stream.Close() }

func normalizeDoQAddr(addr string) (string, string) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, net.JoinHostPort(addr, doqDefaultPort)
	}
	if port == "" || port == "53" {
		return host, net.JoinHostPort(host, doqDefaultPort)
	}
	return host, addr
}

func (s *DNSServer) getDoQConn(ctx context.Context, addr string) (*quic.Conn, error) {
	host, targetAddr := normalizeDoQAddr(addr)

	s.doqMu.Lock()
	if s.doqConns != nil {
		if existing, ok := s.doqConns[targetAddr]; ok && existing.Context().Err() == nil {
			s.doqMu.Unlock()
			return existing, nil
		}
	}
	baseTLS := s.doqTLSConfig
	s.doqMu.Unlock()

	var tlsConf *tls.Config
	if baseTLS != nil {
		tlsConf = baseTLS.Clone()
		tlsConf.NextProtos = []string{doqALPN}
		if tlsConf.ServerName == "" {
			tlsConf.ServerName = host
		}
	} else {
		tlsConf = &tls.Config{
			ServerName: host,
			NextProtos: []string{doqALPN},
			MinVersion: tls.VersionTLS13,
		}
	}

	conn, err := quic.DialAddr(ctx, targetAddr, tlsConf, &quic.Config{
		MaxIdleTimeout:  30 * time.Second,
		KeepAlivePeriod: 15 * time.Second,
	})
	if err != nil {
		return nil, err
	}

	s.doqMu.Lock()
	if s.doqConns == nil {
		s.doqConns = make(map[string]*quic.Conn)
	}
	if old, ok := s.doqConns[targetAddr]; ok && old != conn {
		_ = old.CloseWithError(0, "replaced")
	}
	s.doqConns[targetAddr] = conn
	s.doqMu.Unlock()

	return conn, nil
}

func (s *DNSServer) dropDoQConn(addr string, conn *quic.Conn) {
	_, targetAddr := normalizeDoQAddr(addr)
	s.doqMu.Lock()
	if s.doqConns != nil && s.doqConns[targetAddr] == conn {
		delete(s.doqConns, targetAddr)
	}
	s.doqMu.Unlock()
	if conn != nil {
		_ = conn.CloseWithError(0, "error")
	}
}

func (s *DNSServer) closeDoQConns() {
	s.doqMu.Lock()
	conns := s.doqConns
	s.doqConns = nil
	s.doqMu.Unlock()

	for _, conn := range conns {
		_ = conn.CloseWithError(0, "shutdown")
	}
}

func (s *DNSServer) exchangeDoQ(ctx context.Context, msg *dns.Msg, addr string) (*dns.Msg, error) {
	origID := msg.ID
	wireMsg := msg.Copy()
	// RFC 9250 Section 4.2.1: clients MUST set Message ID to 0 on DoQ wire messages.
	wireMsg.ID = 0
	if err := wireMsg.Pack(); err != nil {
		return nil, err
	}
	if len(wireMsg.Data) > doqMaxMsgSize {
		return nil, fmt.Errorf("DoQ query too large: %d bytes", len(wireMsg.Data))
	}

	var lastErr error
	for attempt := range 2 {
		conn, err := s.getDoQConn(ctx, addr)
		if err != nil {
			return nil, err
		}

		resp, err := doqStreamExchange(ctx, conn, wireMsg.Data)
		if err == nil {
			resp.ID = origID
			return resp, nil
		}

		lastErr = err
		s.dropDoQConn(addr, conn)
		if attempt == 0 && ctx.Err() == nil && !errors.Is(err, context.Canceled) {
			continue
		}
		break
	}

	return nil, lastErr
}

func doqStreamExchange(ctx context.Context, conn *quic.Conn, payload []byte) (*dns.Msg, error) {
	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}

	if deadline, ok := ctx.Deadline(); ok {
		_ = stream.SetDeadline(deadline)
	} else {
		_ = stream.SetDeadline(time.Now().Add(5 * time.Second))
	}

	var lenBuf [2]byte
	// #nosec G115 - payload length is validated against doqMaxMsgSize (65535)
	binary.BigEndian.PutUint16(lenBuf[:], uint16(len(payload)))
	if _, err := stream.Write(lenBuf[:]); err != nil {
		_ = stream.Close()
		return nil, err
	}
	if _, err := stream.Write(payload); err != nil {
		_ = stream.Close()
		return nil, err
	}
	// Half-close write side to signal end of DNS query (RFC 9250 Section 4.2).
	if err := stream.Close(); err != nil {
		return nil, err
	}

	if _, err := io.ReadFull(stream, lenBuf[:]); err != nil {
		return nil, err
	}
	respLen := int(binary.BigEndian.Uint16(lenBuf[:]))
	if respLen == 0 {
		return nil, fmt.Errorf("empty DoQ response")
	}

	respData := make([]byte, respLen)
	if _, err := io.ReadFull(stream, respData); err != nil {
		return nil, err
	}

	response := new(dns.Msg)
	response.Data = respData
	if err := response.Unpack(); err != nil {
		return nil, err
	}

	return response, nil
}
