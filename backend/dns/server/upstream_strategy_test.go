package server

import (
	"context"
	"errors"
	"goaway/backend/settings"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"codeberg.org/miekg/dns"
	"codeberg.org/miekg/dns/rdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeAResponse(name, ip string, rcode uint16) *dns.Msg {
	msg := dns.NewMsg(name, dns.TypeA)
	msg.Response = true
	msg.Rcode = rcode
	if rcode == dns.RcodeSuccess && ip != "" {
		msg.Answer = []dns.RR{
			&dns.A{
				Hdr: dns.Header{Name: name, Class: dns.ClassINET, TTL: 60},
				A:   rdata.A{Addr: netip.MustParseAddr(ip)},
			},
		}
	}
	return msg
}

func TestQueryUpstreamsStrictOrderAndFallback(t *testing.T) {
	srv := &DNSServer{
		Config: &settings.Config{
			DNS: settings.DNSConfig{
				Upstream: settings.UpstreamConfig{
					Strategy: UpstreamStrategyStrict,
					Servers: []settings.UpstreamServer{
						{Name: "primary", Address: "1.1.1.1:53", Protocol: "udp", Enabled: true},
						{Name: "disabled", Address: "9.9.9.9:53", Protocol: "udp", Enabled: false},
						{Name: "secondary", Address: "8.8.8.8:53", Protocol: "udp", Enabled: true},
					},
				},
			},
		},
	}

	var calls []string
	exchange := func(_ context.Context, _ *dns.Msg, addr, _ string) (*dns.Msg, error) {
		calls = append(calls, addr)
		if addr == "1.1.1.1:53" {
			return nil, errors.New("timeout")
		}
		return makeAResponse("example.com.", "2.2.2.2", dns.RcodeSuccess), nil
	}

	q := dns.NewMsg("example.com.", dns.TypeA)
	resp, err := srv.queryUpstreamsWith(context.Background(), q, exchange)
	require.NoError(t, err)
	require.Len(t, resp.Answer, 1)
	assert.Equal(t, []string{"1.1.1.1:53", "8.8.8.8:53"}, calls)
}

func TestQueryUpstreamsParallelBestPicksFastestAndCancelsSlow(t *testing.T) {
	srv := &DNSServer{
		Config: &settings.Config{
			DNS: settings.DNSConfig{
				Upstream: settings.UpstreamConfig{
					Strategy: UpstreamStrategyParallelBest,
					Servers: []settings.UpstreamServer{
						{Name: "slow", Address: "1.1.1.1:53", Protocol: "udp", Enabled: true},
						{Name: "fast", Address: "8.8.8.8:53", Protocol: "udp", Enabled: true},
					},
				},
			},
		},
	}

	var slowCancelled atomic.Bool
	exchange := func(ctx context.Context, _ *dns.Msg, addr, _ string) (*dns.Msg, error) {
		if addr == "1.1.1.1:53" {
			select {
			case <-time.After(300 * time.Millisecond):
				return makeAResponse("example.com.", "1.1.1.1", dns.RcodeSuccess), nil
			case <-ctx.Done():
				slowCancelled.Store(true)
				return nil, ctx.Err()
			}
		}
		time.Sleep(5 * time.Millisecond)
		return makeAResponse("example.com.", "8.8.8.8", dns.RcodeSuccess), nil
	}

	start := time.Now()
	resp, err := srv.queryUpstreamsWith(context.Background(), dns.NewMsg("example.com.", dns.TypeA), exchange)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Len(t, resp.Answer, 1)
	aRec, ok := resp.Answer[0].(*dns.A)
	require.True(t, ok)
	assert.Equal(t, "8.8.8.8", aRec.Addr.String())
	assert.Less(t, elapsed, 150*time.Millisecond)

	assert.Eventually(t, slowCancelled.Load, 100*time.Millisecond, 5*time.Millisecond, "expected slow upstream context to be cancelled")
}

func TestQueryUpstreamsParallelBestSkipsServfailForHealthyPeer(t *testing.T) {
	srv := &DNSServer{
		Config: &settings.Config{
			DNS: settings.DNSConfig{
				Upstream: settings.UpstreamConfig{
					Strategy: UpstreamStrategyParallelBest,
					Servers: []settings.UpstreamServer{
						{Name: "broken", Address: "1.1.1.1:53", Protocol: "udp", Enabled: true},
						{Name: "healthy", Address: "8.8.8.8:53", Protocol: "udp", Enabled: true},
					},
				},
			},
		},
	}

	exchange := func(_ context.Context, _ *dns.Msg, addr, _ string) (*dns.Msg, error) {
		if addr == "1.1.1.1:53" {
			return makeAResponse("example.com.", "", dns.RcodeServerFailure), nil
		}
		time.Sleep(10 * time.Millisecond)
		return makeAResponse("example.com.", "8.8.8.8", dns.RcodeSuccess), nil
	}

	resp, err := srv.queryUpstreamsWith(context.Background(), dns.NewMsg("example.com.", dns.TypeA), exchange)
	require.NoError(t, err)
	assert.Equal(t, uint16(dns.RcodeSuccess), resp.Rcode)
}

func TestQueryUpstreamsRandomDistributesAndFallsBack(t *testing.T) {
	srv := &DNSServer{
		Config: &settings.Config{
			DNS: settings.DNSConfig{
				Upstream: settings.UpstreamConfig{
					Strategy: UpstreamStrategyRandom,
					Servers: []settings.UpstreamServer{
						{Name: "u1", Address: "1.1.1.1:53", Protocol: "udp", Enabled: true},
						{Name: "u2", Address: "8.8.8.8:53", Protocol: "udp", Enabled: true},
					},
				},
			},
		},
	}

	seen := map[string]int{}
	exchange := func(_ context.Context, _ *dns.Msg, addr, _ string) (*dns.Msg, error) {
		seen[addr]++
		return makeAResponse("example.com.", "1.2.3.4", dns.RcodeSuccess), nil
	}

	for range 30 {
		resp, err := srv.queryUpstreamsWith(context.Background(), dns.NewMsg("example.com.", dns.TypeA), exchange)
		require.NoError(t, err)
		require.Len(t, resp.Answer, 1)
	}

	assert.Greater(t, seen["1.1.1.1:53"], 0)
	assert.Greater(t, seen["8.8.8.8:53"], 0)
}
