package server

import (
	"fmt"

	"codeberg.org/miekg/dns"
)

func (s *DNSServer) edeEnabled() bool {
	return s != nil && s.Config != nil && s.Config.DNS.EDEEnabled
}

// attachEDE appends an RFC 8914 Extended DNS Error pseudo-RR to msg when EDE is enabled.
func (s *DNSServer) attachEDE(msg *dns.Msg, code uint16, extraText string) {
	if msg == nil || !s.edeEnabled() {
		return
	}
	if msg.UDPSize == 0 {
		msg.UDPSize = 1232
	}
	msg.Pseudo = append(msg.Pseudo, &dns.EDE{
		InfoCode:  code,
		ExtraText: extraText,
	})
}

func blockedEDEText(category string) string {
	if category != "" {
		return fmt.Sprintf("blocked by goaway (category: %s)", category)
	}
	return "blocked by goaway"
}
