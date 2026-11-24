package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	model "goaway/backend/dns/server/models"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"codeberg.org/miekg/dns"
	"codeberg.org/miekg/dns/dnshttp"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

// DoH3Server serves DNS-over-HTTP/3 (RFC 9114 + RFC 8484) over QUIC.
type DoH3Server struct {
	addr   string
	conn   net.PacketConn
	server *http3.Server
}

// InitDoH3 initializes a DoH3 server bound to dns.address:dns.ports.doh3.
func (s *DNSServer) InitDoH3(cert tls.Certificate) (*DoH3Server, error) {
	return s.InitDoH3WithAddr(fmt.Sprintf("%s:%d", s.Config.DNS.Address, s.Config.DNS.Ports.DoH3), cert, s)
}

// InitDoH3WithAddr creates a DoH3 server on an explicit UDP address and DNS handler.
func (s *DNSServer) InitDoH3WithAddr(addr string, cert tls.Certificate, handler dns.Handler) (*DoH3Server, error) {
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen DoH3 UDP on %s: %w", addr, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(dnshttp.Path, func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		ctx = context.WithValue(ctx, model.DoH3, true)

		msg, err := dnshttp.Request(r)
		if err != nil {
			log.Warning("DoH3 request parsing failed: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		laddr := conn.LocalAddr()
		if v, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr); ok && v != nil {
			laddr = v
		}
		hw := dnshttp.NewResponseWriter(w, r, laddr)
		handler.ServeDNS(ctx, hw, msg)
	})
	mux.HandleFunc("/health", s.handleHealthCheck)

	srv := &http3.Server{
		Addr:    conn.LocalAddr().String(),
		Handler: mux,
		TLSConfig: http3.ConfigureTLSConfig(&tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS13,
		}),
		QUICConfig: &quic.Config{
			MaxIdleTimeout:  30 * time.Second,
			KeepAlivePeriod: 15 * time.Second,
		},
		MaxHeaderBytes: 1 * megabyte,
		IdleTimeout:    60 * time.Second,
	}

	return &DoH3Server{
		addr:   conn.LocalAddr().String(),
		conn:   conn,
		server: srv,
	}, nil
}

// Addr returns the bound UDP address of the DoH3 server.
func (d *DoH3Server) Addr() string {
	return d.addr
}

// ListenAndServe serves HTTP/3 requests until Shutdown or error.
func (d *DoH3Server) ListenAndServe() error {
	log.Info("Started DoH3 (dns-over-http3) server on https://%s%s", d.addr, dnshttp.Path)
	err := d.server.Serve(d.conn)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Shutdown gracefully stops the HTTP/3 server and closes the UDP socket.
func (d *DoH3Server) Shutdown(ctx context.Context) error {
	err := d.server.Shutdown(ctx)
	if d.conn != nil {
		_ = d.conn.Close()
	}
	return err
}

func newDoH3HTTPClient(baseTLS *tls.Config) (*http.Client, *http3.Transport) {
	var tlsConf *tls.Config
	if baseTLS != nil {
		tlsConf = baseTLS.Clone()
	} else {
		tlsConf = &tls.Config{MinVersion: tls.VersionTLS13}
	}

	transport := &http3.Transport{
		TLSClientConfig: tlsConf,
		QUICConfig: &quic.Config{
			MaxIdleTimeout:  30 * time.Second,
			KeepAlivePeriod: 15 * time.Second,
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}, transport
}

func (s *DNSServer) getDoH3Client() *http.Client {
	s.doh3ClientOnce.Do(func() {
		if s.doh3Client == nil {
			s.doh3Client, s.doh3Transport = newDoH3HTTPClient(s.doh3TLSConfig)
		}
	})
	return s.doh3Client
}

func (s *DNSServer) closeDoH3Transport() {
	if s.doh3Transport != nil {
		_ = s.doh3Transport.Close()
	}
}

func normalizeDoH3URL(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "h3://") {
		raw = "https://" + strings.TrimPrefix(raw, "h3://")
	} else if strings.HasPrefix(raw, "doh3://") {
		raw = "https://" + strings.TrimPrefix(raw, "doh3://")
	} else if !strings.HasPrefix(raw, "https://") {
		raw = "https://" + raw
	}
	return raw
}

func (s *DNSServer) exchangeDoH3(ctx context.Context, msg *dns.Msg, targetURL string) (*dns.Msg, error) {
	origID := msg.ID
	wireMsg := msg.Copy()
	wireMsg.ID = 0 // RFC 8484 §4.1: DoH requests SHOULD use DNS message ID 0
	if err := wireMsg.Pack(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, normalizeDoH3URL(targetURL), bytes.NewReader(wireMsg.Data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")

	resp, err := s.getDoH3Client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("DoH3 server returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return nil, err
	}

	response := new(dns.Msg)
	response.Data = body
	if err := response.Unpack(); err != nil {
		return nil, err
	}
	response.ID = origID
	return response, nil
}
