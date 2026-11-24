package server

import (
	"context"
	"crypto/tls"
	model "goaway/backend/dns/server/models"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"codeberg.org/miekg/dns"
	"codeberg.org/miekg/dns/dnshttp"
	"codeberg.org/miekg/dns/rdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoH3ServerAndUpstreamRoundTrip(t *testing.T) {
	cert, pool := generateSelfSignedCert(t)
	var sawDoH3Ctx atomic.Bool

	handler := dns.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) {
		if ctx.Value(model.DoH3) == true {
			sawDoH3Ctx.Store(true)
		}

		resp := r.Copy()
		resp.Response = true
		resp.Answer = []dns.RR{
			&dns.A{
				Hdr: dns.Header{Name: r.Question[0].Header().Name, Class: dns.ClassINET, TTL: 90},
				A:   rdata.A{Addr: netip.MustParseAddr("4.3.2.1")},
			},
		}
		if err := resp.Pack(); err != nil {
			return
		}
		_, _ = resp.WriteTo(w)
	})

	srv := &DNSServer{
		doh3TLSConfig: &tls.Config{
			RootCAs:    pool,
			ServerName: "127.0.0.1",
			MinVersion: tls.VersionTLS13,
		},
	}

	doh3Srv, err := srv.InitDoH3WithAddr("127.0.0.1:0", cert, handler)
	require.NoError(t, err)
	go func() { _ = doh3Srv.ListenAndServe() }()
	t.Cleanup(func() {
		_ = doh3Srv.Shutdown(context.Background())
		srv.CloseIdleConnections()
	})

	endpoint := "https://" + doh3Srv.Addr() + dnshttp.Path
	for i := range 3 {
		q := dns.NewMsg("doh3.example.com.", dns.TypeA)
		q.ID = uint16(2000 + i)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		resp, err := srv.exchangeWithProtocolContext(ctx, q, endpoint, "doh3")
		cancel()

		require.NoError(t, err)
		assert.Equal(t, q.ID, resp.ID)
		require.Len(t, resp.Answer, 1)
		aRec, ok := resp.Answer[0].(*dns.A)
		require.True(t, ok)
		assert.Equal(t, "4.3.2.1", aRec.Addr.String())
	}

	assert.True(t, sawDoH3Ctx.Load(), "expected DoH3 context marker to be set by DoH3Server")
}

func TestNormalizeDoH3URL(t *testing.T) {
	assert.Equal(t, "https://dns.google/dns-query", normalizeDoH3URL("h3://dns.google/dns-query"))
	assert.Equal(t, "https://cloudflare-dns.com/dns-query", normalizeDoH3URL("doh3://cloudflare-dns.com/dns-query"))
	assert.Equal(t, "https://dns.quad9.net/dns-query", normalizeDoH3URL("https://dns.quad9.net/dns-query"))
	assert.Equal(t, "https://1.1.1.1/dns-query", normalizeDoH3URL("1.1.1.1/dns-query"))
}
