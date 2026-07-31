package server

import (
	"testing"

	"codeberg.org/miekg/dns"
)

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.1.1", true},
		{"172.15.0.1", false}, // just below the 172.16/12 block
		{"172.32.0.1", false}, // just above the 172.16/12 block
		{"8.8.8.8", false},
		{"127.0.0.1", false}, // loopback is not RFC1918
		{"not-an-ip", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			if got := isPrivateIP(tt.ip); got != tt.want {
				t.Errorf("isPrivateIP(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestTrimDomainDot(t *testing.T) {
	tests := map[string]string{
		"example.com.":  "example.com",
		"example.com":   "example.com",
		".":             "",
		"":              "",
		"a.b.c.d.e.f.g": "a.b.c.d.e.f.g",
	}

	for in, want := range tests {
		if got := trimDomainDot(in); got != want {
			t.Errorf("trimDomainDot(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsLocalLookup(t *testing.T) {
	tests := map[string]bool{
		"1.0.0.10.in-addr.arpa.": true,
		"8.b.d.0.ip6.arpa.":      true,
		"example.com.":           false,
		"in-addr.arpa.":          false, // needs a label in front
	}

	for qname, want := range tests {
		if got := isLocalLookup(qname); got != want {
			t.Errorf("isLocalLookup(%q) = %v, want %v", qname, got, want)
		}
	}
}

func TestIsPTRQuery(t *testing.T) {
	tests := []struct {
		name  string
		qname string
		qtype uint16
		want  bool
	}{
		{"PTR type", "example.com.", dns.TypePTR, true},
		{"reverse name with A type", "1.0.0.10.in-addr.arpa.", dns.TypeA, true},
		{"plain A query", "example.com.", dns.TypeA, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := dns.NewMsg(tt.qname, tt.qtype)
			req := &Request{Msg: msg, Question: msg.Question[0]}

			if got := isPTRQuery(req, tt.qname); got != tt.want {
				t.Errorf("isPTRQuery(%q, %d) = %v, want %v", tt.qname, tt.qtype, got, tt.want)
			}
		})
	}
}
