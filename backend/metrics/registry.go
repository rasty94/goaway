package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	TotalQueries = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "goaway_dns_queries_total",
			Help: "Total number of DNS queries received",
		},
		[]string{"client_ip", "type"},
	)

	BlockedQueries = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "goaway_dns_blocked_total",
			Help: "Total number of DNS queries blocked",
		},
		[]string{"client_ip", "domain"},
	)

	ThrottledQueries = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "goaway_dns_throttled_total",
			Help: "Total number of DNS queries throttled by per-client rate limits",
		},
		[]string{"client_ip", "protocol"},
	)

	CachedQueries = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "goaway_dns_cached_total",
			Help: "Total number of DNS queries answered from cache",
		},
		[]string{"client_ip", "domain"},
	)

	ForwardedQueries = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "goaway_dns_forwarded_total",
			Help: "Total number of DNS queries forwarded to upstream",
		},
		[]string{"client_ip", "upstream"},
	)

	DNSLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "goaway_dns_latency_seconds",
			Help:    "Latency of DNS query resolution",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"client_ip", "status"},
	)

	DNSSECResponses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "goaway_dns_dnssec_total",
			Help: "Total DNS responses grouped by DNSSEC status",
		},
		[]string{"client_ip", "dnssec_status"},
	)
)

func init() {
	prometheus.MustRegister(TotalQueries)
	prometheus.MustRegister(BlockedQueries)
	prometheus.MustRegister(ThrottledQueries)
	prometheus.MustRegister(CachedQueries)
	prometheus.MustRegister(ForwardedQueries)
	prometheus.MustRegister(DNSLatency)
	prometheus.MustRegister(DNSSECResponses)
}
