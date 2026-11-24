package server

import (
	"context"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"codeberg.org/miekg/dns"
	"codeberg.org/miekg/dns/rdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDoHServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var newConns atomic.Int32

	ts := httptest.NewUnstartedServer(handler)
	ts.EnableHTTP2 = true
	ts.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			newConns.Add(1)
		}
	}
	ts.StartTLS()
	t.Cleanup(ts.Close)

	return ts, &newConns
}

func newDNSServerForDoHTest(t *testing.T, ts *httptest.Server) *DNSServer {
	t.Helper()
	client := newDoHHTTPClient()
	tr, ok := client.Transport.(*http.Transport)
	require.True(t, ok)

	pool := x509.NewCertPool()
	pool.AddCert(ts.Certificate())
	tr.TLSClientConfig.RootCAs = pool

	return &DNSServer{dohClient: client}
}

func TestExchangeDoHReusesHTTP2Connection(t *testing.T) {
	var seenProto atomic.Value

	ts, newConns := newTestDoHServer(t, func(w http.ResponseWriter, r *http.Request) {
		seenProto.Store(r.Proto)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/dns-message", r.Header.Get("Content-Type"))
		assert.Equal(t, "application/dns-message", r.Header.Get("Accept"))

		body, err := io.ReadAll(r.Body)
		if !assert.NoError(t, err) {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}

		reqMsg := new(dns.Msg)
		reqMsg.Data = body
		if !assert.NoError(t, reqMsg.Unpack()) {
			http.Error(w, "bad dns", http.StatusBadRequest)
			return
		}

		respMsg := reqMsg.Copy()
		respMsg.Response = true
		respMsg.Answer = []dns.RR{
			&dns.A{
				Hdr: dns.Header{Name: reqMsg.Question[0].Header().Name, Class: dns.ClassINET, TTL: 60},
				A:   rdata.A{Addr: netip.MustParseAddr("1.2.3.4")},
			},
		}
		if !assert.NoError(t, respMsg.Pack()) {
			http.Error(w, "pack failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/dns-message")
		_, _ = w.Write(respMsg.Data)
	})

	srv := newDNSServerForDoHTest(t, ts)
	defer srv.CloseIdleConnections()

	const queries = 5
	for range queries {
		q := dns.NewMsg("example.com.", dns.TypeA)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		resp, err := srv.exchangeDoH(ctx, q, ts.URL)
		cancel()

		require.NoError(t, err)
		require.Len(t, resp.Answer, 1)
	}

	assert.Equal(t, "HTTP/2.0", seenProto.Load())
	assert.Equal(t, int32(1), newConns.Load(), "expected all DoH queries to reuse a single HTTP/2 connection")
}

func TestExchangeDoHNon200Status(t *testing.T) {
	ts, _ := newTestDoHServer(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream failure", http.StatusBadGateway)
	})

	srv := newDNSServerForDoHTest(t, ts)
	defer srv.CloseIdleConnections()

	q := dns.NewMsg("example.com.", dns.TypeA)
	_, err := srv.exchangeDoH(context.Background(), q, ts.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 502")
}

func TestCloseIdleConnectionsZeroValueSafe(t *testing.T) {
	var srv DNSServer
	assert.NotPanics(t, func() {
		srv.CloseIdleConnections()
	})
}
