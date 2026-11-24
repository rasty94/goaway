package server

import (
	"context"
	"fmt"
	"goaway/backend/metrics"
	"goaway/backend/settings"
	"math/rand/v2"
	"strings"
	"time"

	"codeberg.org/miekg/dns"
)

const (
	UpstreamStrategyStrict       = "strict"
	UpstreamStrategyParallelBest = "parallel_best"
	UpstreamStrategyRandom       = "random"

	parallelBestMaxFanout = 3
)

type upstreamExchangeFunc func(ctx context.Context, msg *dns.Msg, addr, proto string) (*dns.Msg, error)

func (s *DNSServer) upstreamStrategy() string {
	if s == nil || s.Config == nil {
		return UpstreamStrategyStrict
	}
	dnsCfg := s.Config.DNSSnapshot()
	switch strings.ToLower(strings.TrimSpace(dnsCfg.Upstream.Strategy)) {
	case UpstreamStrategyParallelBest:
		return UpstreamStrategyParallelBest
	case UpstreamStrategyRandom:
		return UpstreamStrategyRandom
	default:
		return UpstreamStrategyStrict
	}
}

func (s *DNSServer) enabledUpstreams() []settings.UpstreamServer {
	if s == nil || s.Config == nil {
		return nil
	}
	servers := s.Config.DNSSnapshot().Upstream.Servers
	enabled := make([]settings.UpstreamServer, 0, len(servers))
	for _, u := range servers {
		if u.Enabled {
			enabled = append(enabled, u)
		}
	}
	return enabled
}

func (s *DNSServer) queryUpstreams(ctx context.Context, baseMsg *dns.Msg) (*dns.Msg, error) {
	return s.queryUpstreamsWith(ctx, baseMsg, s.exchangeWithProtocolContext)
}

func (s *DNSServer) queryUpstreamsWith(ctx context.Context, baseMsg *dns.Msg, exchange upstreamExchangeFunc) (*dns.Msg, error) {
	upstreams := s.enabledUpstreams()
	if len(upstreams) == 0 {
		return nil, fmt.Errorf("no enabled upstream servers configured")
	}

	strategy := s.upstreamStrategy()
	switch strategy {
	case UpstreamStrategyParallelBest:
		return s.queryParallelBest(ctx, baseMsg, upstreams, exchange)
	case UpstreamStrategyRandom:
		shuffled := make([]settings.UpstreamServer, len(upstreams))
		copy(shuffled, upstreams)
		// #nosec G404 - non-cryptographic shuffle for upstream load distribution
		rand.Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})
		return s.querySequential(ctx, baseMsg, shuffled, strategy, exchange)
	default:
		return s.querySequential(ctx, baseMsg, upstreams, UpstreamStrategyStrict, exchange)
	}
}

func (s *DNSServer) querySequential(ctx context.Context, baseMsg *dns.Msg, upstreams []settings.UpstreamServer, strategy string, exchange upstreamExchangeFunc) (*dns.Msg, error) {
	var lastErr error
	for _, upstream := range upstreams {
		log.Debug("Sending query to '%s' (%s) using %s", upstream.Name, upstream.Address, upstream.Protocol)
		start := time.Now()
		msgCopy := baseMsg.Copy()
		in, err := exchange(ctx, msgCopy, upstream.Address, upstream.Protocol)
		metrics.UpstreamLatency.WithLabelValues(upstream.Address, upstream.Protocol).Observe(time.Since(start).Seconds())
		if err == nil && in != nil {
			metrics.UpstreamWins.WithLabelValues(upstream.Address, upstream.Protocol, strategy).Inc()
			return in, nil
		}
		lastErr = err
		log.Warning("Upstream '%s' failed: %v", upstream.Name, err)
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no response from any upstream")
}

func (s *DNSServer) queryParallelBest(ctx context.Context, baseMsg *dns.Msg, upstreams []settings.UpstreamServer, exchange upstreamExchangeFunc) (*dns.Msg, error) {
	if len(upstreams) == 1 {
		return s.querySequential(ctx, baseMsg, upstreams, UpstreamStrategyParallelBest, exchange)
	}

	fanout := min(len(upstreams), parallelBestMaxFanout)
	candidates := upstreams[:fanout]

	type result struct {
		upstream settings.UpstreamServer
		msg      *dns.Msg
		err      error
		latency  time.Duration
	}

	raceCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	ch := make(chan result, len(candidates))
	for _, u := range candidates {
		upstream := u
		msgCopy := baseMsg.Copy()
		go func() {
			start := time.Now()
			in, err := exchange(raceCtx, msgCopy, upstream.Address, upstream.Protocol)
			ch <- result{
				upstream: upstream,
				msg:      in,
				err:      err,
				latency:  time.Since(start),
			}
		}()
	}

	var lastErr error
	var fallbackResp *dns.Msg
	var fallbackUpstream settings.UpstreamServer

	for range len(candidates) {
		res := <-ch
		metrics.UpstreamLatency.WithLabelValues(res.upstream.Address, res.upstream.Protocol).Observe(res.latency.Seconds())
		if res.err == nil && res.msg != nil {
			if res.msg.Rcode != dns.RcodeServerFailure {
				cancel()
				metrics.UpstreamWins.WithLabelValues(res.upstream.Address, res.upstream.Protocol, UpstreamStrategyParallelBest).Inc()
				return res.msg, nil
			}
			// Keep SERVFAIL as a fallback in case all upstreams return SERVFAIL.
			if fallbackResp == nil {
				fallbackResp = res.msg
				fallbackUpstream = res.upstream
			}
			continue
		}
		lastErr = res.err
		log.Warning("Upstream '%s' failed in parallel_best: %v", res.upstream.Name, res.err)
	}

	if fallbackResp != nil {
		metrics.UpstreamWins.WithLabelValues(fallbackUpstream.Address, fallbackUpstream.Protocol, UpstreamStrategyParallelBest).Inc()
		return fallbackResp, nil
	}

	if fanout < len(upstreams) {
		return s.querySequential(ctx, baseMsg, upstreams[fanout:], UpstreamStrategyParallelBest, exchange)
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no response from any upstream")
}
