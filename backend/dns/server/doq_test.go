package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	model "goaway/backend/dns/server/models"
	"math/big"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"codeberg.org/miekg/dns"
	"codeberg.org/miekg/dns/rdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateSelfSignedCert(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(priv)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)

	parsed, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(parsed)

	return cert, pool
}

func TestDoQServerAndUpstreamRoundTripAndSessionReuse(t *testing.T) {
	cert, pool := generateSelfSignedCert(t)
	var sawDoQCtx atomic.Bool
	var sawZeroWireID atomic.Bool

	handler := dns.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) {
		if ctx.Value(model.DoQ) == true {
			sawDoQCtx.Store(true)
		}
		if r.ID == 0 {
			sawZeroWireID.Store(true)
		}

		resp := r.Copy()
		resp.Response = true
		resp.Answer = []dns.RR{
			&dns.A{
				Hdr: dns.Header{Name: r.Question[0].Header().Name, Class: dns.ClassINET, TTL: 120},
				A:   rdata.A{Addr: netip.MustParseAddr("9.8.7.6")},
			},
		}
		if err := resp.Pack(); err != nil {
			return
		}
		_, _ = w.Write(resp.Data)
	})

	srv := &DNSServer{
		doqTLSConfig: &tls.Config{
			RootCAs:    pool,
			ServerName: "127.0.0.1",
			MinVersion: tls.VersionTLS13,
		},
	}

	doqSrv, err := srv.InitDoQWithAddr("127.0.0.1:0", cert, handler)
	require.NoError(t, err)
	go func() { _ = doqSrv.ListenAndServe() }()
	t.Cleanup(func() {
		_ = doqSrv.Shutdown(context.Background())
		srv.CloseIdleConnections()
	})

	for i := range 3 {
		q := dns.NewMsg("doq.example.com.", dns.TypeA)
		q.ID = uint16(1000 + i)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		resp, err := srv.exchangeWithProtocolContext(ctx, q, doqSrv.Addr(), "doq")
		cancel()

		require.NoError(t, err)
		assert.Equal(t, uint16(1000+i), resp.ID, "original DNS message ID must be restored")
		require.Len(t, resp.Answer, 1)
		aRec, ok := resp.Answer[0].(*dns.A)
		require.True(t, ok)
		assert.Equal(t, "9.8.7.6", aRec.Addr.String())
	}

	assert.True(t, sawDoQCtx.Load(), "expected DoQ context value to be set by DoQServer")
	assert.True(t, sawZeroWireID.Load(), "expected RFC 9250 wire ID == 0")

	srv.doqMu.Lock()
	connCount := len(srv.doqConns)
	srv.doqMu.Unlock()
	assert.Equal(t, 1, connCount, "expected QUIC session to be pooled and reused")
}
