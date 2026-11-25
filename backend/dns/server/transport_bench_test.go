package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync"
	"testing"
	"time"

	"codeberg.org/miekg/dns"
	"codeberg.org/miekg/dns/dnshttp"
	"codeberg.org/miekg/dns/rdata"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// Transport benchmarks and leak checks for every listener goaway exposes.
// The handler is a stub so the numbers measure protocol overhead only; the
// full resolver path is exercised by the soak tool (test/soak, docs/SOAK.md).

var stubHandler = dns.HandlerFunc(func(_ context.Context, w dns.ResponseWriter, r *dns.Msg) {
	resp := r.Copy()
	resp.Response = true
	resp.Answer = []dns.RR{&dns.A{
		Hdr: dns.Header{Name: r.Question[0].Header().Name, Class: dns.ClassINET, TTL: 60},
		A:   rdata.A{Addr: netip.MustParseAddr("192.0.2.1")},
	}}
	if err := resp.Pack(); err != nil {
		return
	}
	_, _ = io.Copy(w, resp) // same write path as Request.Respond
})

type exchangeFunc func(ctx context.Context, m *dns.Msg) (*dns.Msg, error)

// transport is a running server plus a client. Stateless clients set exchange
// (safe for concurrent use); connection-oriented ones set newExchange, which
// returns a client owned by a single goroutine.
type transport struct {
	exchange    exchangeFunc
	newExchange func() exchangeFunc
	stop        func()
}

func (tr transport) client() exchangeFunc {
	if tr.newExchange != nil {
		return tr.newExchange()
	}
	return tr.exchange
}

var transportProtocols = []string{"udp", "tcp", "dot", "doh", "doq", "doh3"}

func startTransport(tb testing.TB, proto string) transport {
	tb.Helper()
	cert, pool := generateSelfSignedCert(tb)
	clientTLS := &tls.Config{RootCAs: pool, ServerName: "127.0.0.1", MinVersion: tls.VersionTLS12}

	switch proto {
	case "udp", "tcp", "dot":
		srv := &dns.Server{Handler: stubHandler, ReadTimeout: 5 * time.Second}
		var addr string
		if proto == "udp" {
			pc, err := net.ListenPacket("udp", "127.0.0.1:0")
			require.NoError(tb, err)
			srv.Net, srv.PacketConn, addr = "udp", pc, pc.LocalAddr().String()
		} else {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(tb, err)
			if proto == "dot" {
				ln = tls.NewListener(ln, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
			}
			srv.Net, srv.Listener, addr = "tcp", ln, ln.Addr().String()
		}
		done := make(chan struct{})
		go func() { _ = srv.ListenAndServe(); close(done) }()

		client := dns.NewClient()
		if proto == "udp" {
			return transport{
				exchange: func(ctx context.Context, m *dns.Msg) (*dns.Msg, error) {
					in, _, err := client.Exchange(ctx, m, "udp", addr)
					return in, err
				},
				stop: func() { srv.Shutdown(context.Background()); <-done },
			}
		}
		// TCP/DoT: one persistent connection per client, like real stub
		// resolvers; dialing per query exhausts ephemeral ports in TIME_WAIT.
		var mu sync.Mutex
		var conns []net.Conn
		return transport{
			newExchange: func() exchangeFunc {
				var conn net.Conn
				return func(ctx context.Context, m *dns.Msg) (in *dns.Msg, err error) {
					// The server closes a connection after MaxTCPQueries or on idle,
					// so redial once like a real client would.
					for range 2 {
						if conn == nil {
							if proto == "dot" {
								conn, err = tls.Dial("tcp", addr, clientTLS)
							} else {
								conn, err = net.Dial("tcp", addr)
							}
							if err != nil {
								return nil, err
							}
							mu.Lock()
							conns = append(conns, conn)
							mu.Unlock()
						}
						if in, _, err = client.ExchangeWithConn(ctx, m.Copy(), conn); err == nil {
							return in, nil
						}
						_ = conn.Close()
						conn = nil
					}
					return nil, err
				}
			},
			stop: func() {
				mu.Lock()
				for _, c := range conns {
					_ = c.Close()
				}
				mu.Unlock()
				srv.Shutdown(context.Background())
				<-done
			},
		}

	case "doh":
		mux := http.NewServeMux()
		mux.HandleFunc(dnshttp.Path, func(w http.ResponseWriter, r *http.Request) {
			msg, err := dnshttp.Request(r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			laddr := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
			stubHandler.ServeDNS(r.Context(), dnshttp.NewResponseWriter(w, r, laddr), msg)
		})
		ts := httptest.NewUnstartedServer(mux)
		ts.EnableHTTP2 = true
		ts.StartTLS()
		client := newDoHHTTPClient()
		roots := x509.NewCertPool()
		roots.AddCert(ts.Certificate())
		client.Transport.(*http.Transport).TLSClientConfig.RootCAs = roots
		s := &DNSServer{dohClient: client}
		url := ts.URL + dnshttp.Path
		return transport{
			exchange: func(ctx context.Context, m *dns.Msg) (*dns.Msg, error) { return s.exchangeDoH(ctx, m, url) },
			stop:     func() { s.CloseIdleConnections(); ts.Close() },
		}

	case "doq":
		s := &DNSServer{doqTLSConfig: clientTLS}
		srv, err := s.InitDoQWithAddr("127.0.0.1:0", cert, stubHandler)
		require.NoError(tb, err)
		done := make(chan struct{})
		go func() { _ = srv.ListenAndServe(); close(done) }()
		return transport{
			exchange: func(ctx context.Context, m *dns.Msg) (*dns.Msg, error) {
				return s.exchangeWithProtocolContext(ctx, m, srv.Addr(), "doq")
			},
			stop: func() { s.CloseIdleConnections(); _ = srv.Shutdown(context.Background()); <-done },
		}

	case "doh3":
		s := &DNSServer{doh3TLSConfig: clientTLS}
		srv, err := s.InitDoH3WithAddr("127.0.0.1:0", cert, stubHandler)
		require.NoError(tb, err)
		done := make(chan struct{})
		go func() { _ = srv.ListenAndServe(); close(done) }()
		url := "https://" + srv.Addr() + dnshttp.Path
		return transport{
			exchange: func(ctx context.Context, m *dns.Msg) (*dns.Msg, error) {
				return s.exchangeWithProtocolContext(ctx, m, url, "doh3")
			},
			stop: func() { s.CloseIdleConnections(); _ = srv.Shutdown(context.Background()); <-done },
		}
	}
	tb.Fatalf("unknown protocol %q", proto)
	return transport{}
}

func query(tb testing.TB, exchange exchangeFunc) {
	q := dns.NewMsg("bench.example.com.", dns.TypeA)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := exchange(ctx, q)
	if err != nil {
		tb.Errorf("exchange: %v", err) // Errorf: safe from RunParallel goroutines
		return
	}
	if len(resp.Answer) != 1 {
		tb.Errorf("expected 1 answer, got %d", len(resp.Answer))
	}
}

// TestTransportStartStopNoLeaks starts each listener, serves queries, stops it
// and verifies no goroutine outlives the shutdown.
func TestTransportStartStopNoLeaks(t *testing.T) {
	for _, proto := range transportProtocols {
		t.Run(proto, func(t *testing.T) {
			defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
			tr := startTransport(t, proto)
			defer tr.stop() // runs before VerifyNone (LIFO)
			exchange := tr.client()
			for range 5 {
				query(t, exchange)
			}
		})
	}
}

func BenchmarkTransport(b *testing.B) {
	for _, proto := range transportProtocols {
		b.Run(proto, func(b *testing.B) {
			tr := startTransport(b, proto)
			defer tr.stop()
			exchange := tr.client()
			b.ReportAllocs()
			for b.Loop() {
				query(b, exchange)
			}
		})
	}
}

func BenchmarkTransportParallel(b *testing.B) {
	for _, proto := range transportProtocols {
		b.Run(proto, func(b *testing.B) {
			tr := startTransport(b, proto)
			defer tr.stop()
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				exchange := tr.client()
				for pb.Next() {
					query(b, exchange)
				}
			})
		})
	}
}
