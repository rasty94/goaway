package server

import (
	model "goaway/backend/dns/server/models"
	"goaway/backend/metrics"
	"time"

	"github.com/miekg/dns"
)

type clientRateLimitWindow struct {
	attempts     []time.Time
	blockedUntil time.Time
}

type dnsRateLimitConfig struct {
	enabled       bool
	maxQueries    int
	window        time.Duration
	blockDuration time.Duration
}

func (s *DNSServer) getDNSRateLimitConfig() dnsRateLimitConfig {
	raw := s.Config.DNS.RateLimit

	cfg := dnsRateLimitConfig{
		enabled: raw.Enabled,
	}

	if raw.MaxQueries <= 0 {
		cfg.maxQueries = 120
	} else {
		cfg.maxQueries = raw.MaxQueries
	}

	if raw.WindowSeconds <= 0 {
		cfg.window = 10 * time.Second
	} else {
		cfg.window = time.Duration(raw.WindowSeconds) * time.Second
	}

	if raw.BlockDurationSeconds <= 0 {
		cfg.blockDuration = 30 * time.Second
	} else {
		cfg.blockDuration = time.Duration(raw.BlockDurationSeconds) * time.Second
	}

	return cfg
}

func (s *DNSServer) isDNSRateLimited(clientIP string) (bool, int) {
	cfg := s.getDNSRateLimitConfig()
	if !cfg.enabled {
		return false, 0
	}

	now := time.Now()
	cutoff := now.Add(-cfg.window)

	s.rateLimitLock.Lock()
	defer s.rateLimitLock.Unlock()

	entry, ok := s.clientRateLimitCache[clientIP]
	if !ok {
		entry = &clientRateLimitWindow{}
		s.clientRateLimitCache[clientIP] = entry
	}

	if entry.blockedUntil.After(now) {
		remaining := int(entry.blockedUntil.Sub(now).Seconds())
		if remaining < 1 {
			remaining = 1
		}
		return true, remaining
	}

	kept := make([]time.Time, 0, len(entry.attempts)+1)
	for _, attempt := range entry.attempts {
		if attempt.After(cutoff) {
			kept = append(kept, attempt)
		}
	}

	if len(kept) >= cfg.maxQueries {
		entry.attempts = kept
		entry.blockedUntil = now.Add(cfg.blockDuration)
		return true, int(cfg.blockDuration.Seconds())
	}

	entry.attempts = append(kept, now)
	return false, 0
}

func (s *DNSServer) writeRateLimitedResponse(req *Request, _ int) model.RequestLogEntry {
	msg := new(dns.Msg)
	msg.SetReply(req.Msg)
	msg.Authoritative = false
	msg.RecursionAvailable = true
	msg.Rcode = dns.RcodeRefused

	_ = req.ResponseWriter.WriteMsg(msg)
	metrics.ThrottledQueries.WithLabelValues(req.Client.IP, string(req.Protocol)).Inc()

	return model.RequestLogEntry{
		Domain:            req.Question.Name,
		Status:            dns.RcodeToString[dns.RcodeRefused],
		QueryType:         dns.TypeToString[req.Question.Qtype],
		IP:                nil,
		ResponseSizeBytes: msg.Len(),
		Timestamp:         req.Sent,
		ResponseTime:      time.Since(req.Sent),
		Blocked:           true,
		Cached:            false,
		ClientInfo:        req.Client,
		Protocol:          req.Protocol,
	}
}
