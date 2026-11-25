package server

import (
	"net/netip"
	"testing"

	"goaway/backend/settings"

	"codeberg.org/miekg/dns"
	"codeberg.org/miekg/dns/rdata"
	"github.com/stretchr/testify/require"
)

// Cache hits must answer NOERROR. The handler maps the status string back with
// dns.StringToRcode, so it has to be an rcode name ("NOERROR"), not the EDNS0
// option-code name dnsutil.CodeToString returns ("CODE0" -> SERVFAIL).
func TestResolveCacheHitStatusIsNOERROR(t *testing.T) {
	cfg := &settings.Config{}
	cfg.DNS.CacheEnabled = true
	cfg.DNS.CacheTTL = 60
	s := &DNSServer{Config: cfg}

	q := dns.NewMsg("cached.example.org.", dns.TypeA)
	req := &Request{Msg: q, Question: q.Question[0]}
	rr := &dns.A{
		Hdr: dns.Header{Name: "cached.example.org.", Class: dns.ClassINET, TTL: 60},
		A:   rdata.A{Addr: netip.MustParseAddr("192.0.2.1")},
	}
	s.CacheRecord(req.QName()+":"+req.QTypeStr(), req.QName(), []dns.RR{rr}, 60, "")

	answers, cached, _, _, status, _ := s.Resolve(req)
	require.True(t, cached)
	require.Len(t, answers, 1)
	rcode, ok := dns.StringToRcode[status]
	require.True(t, ok, "status %q is not an rcode name", status)
	require.Equal(t, uint16(dns.RcodeSuccess), rcode)
}
