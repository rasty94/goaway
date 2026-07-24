package server

import (
	"fmt"
	model "goaway/backend/dns/server/models"
	"strings"
	"time"

	"codeberg.org/miekg/dns"
	"codeberg.org/miekg/dns/dnsutil"
)

const (
	DNSSECStatusDisabled = "disabled"
	DNSSECStatusSecure   = "secure"
	DNSSECStatusInsecure = "insecure"
	DNSSECStatusBogus    = "bogus"
)

func (s *DNSServer) dnssecMode() string {
	if !s.Config.DNS.DNSSEC.Enabled {
		return "off"
	}

	mode := strings.ToLower(strings.TrimSpace(s.Config.DNS.DNSSEC.Mode))
	switch mode {
	case "strict", "permissive":
		return mode
	default:
		return "permissive"
	}
}

func (s *DNSServer) classifyDNSSECResponse(msg *dns.Msg, queryErr error) string {
	mode := s.dnssecMode()
	if mode == "off" {
		return DNSSECStatusDisabled
	}

	if queryErr != nil {
		if mode == "strict" {
			return DNSSECStatusBogus
		}
		return DNSSECStatusInsecure
	}

	if msg == nil {
		if mode == "strict" {
			return DNSSECStatusBogus
		}
		return DNSSECStatusInsecure
	}

	if msg.AuthenticatedData {
		return DNSSECStatusSecure
	}

	if mode == "strict" {
		return DNSSECStatusBogus
	}

	return DNSSECStatusInsecure
}

func (s *DNSServer) defaultDNSSECStatus() string {
	if s.dnssecMode() == "off" {
		return DNSSECStatusDisabled
	}
	return DNSSECStatusInsecure
}

type DNSSECDiagnostic struct {
	Domain       string   `json:"domain"`
	Type         string   `json:"type"`
	Status       string   `json:"status"`
	DNSSECStatus string   `json:"dnssecStatus"`
	AnswerCount  int      `json:"answerCount"`
	AuthorityRRs int      `json:"authorityCount"`
	ExtraRRs     int      `json:"extraCount"`
	AD           bool     `json:"authenticatedData"`
	DO           bool     `json:"dnssecOk"`
	Answers      []string `json:"answers"`
}

func (s *DNSServer) DiagnoseDNSSEC(domain string, qtype uint16) (*DNSSECDiagnostic, error) {
	if strings.TrimSpace(domain) == "" {
		return nil, fmt.Errorf("domain is required")
	}

	msg := dns.NewMsg(dnsutil.Fqdn(strings.TrimSpace(domain)), qtype)

	req := &Request{
		Sent:     time.Now(),
		Msg:      msg,
		Question: msg.Question[0],
		Client:   &model.Client{IP: IPv4Loopback, Name: "diagnostic"},
		Protocol: model.UDP,
	}

	answers, _, status, dnssecStatus := s.QueryUpstream(req)

	answerStrings := make([]string, 0, len(answers))
	for _, rr := range answers {
		answerStrings = append(answerStrings, rr.String())
	}

	diagnostic := &DNSSECDiagnostic{
		Domain:       dnsutil.Fqdn(strings.TrimSpace(domain)),
		Type:         dns.TypeToString[qtype],
		Status:       status,
		DNSSECStatus: dnssecStatus,
		AnswerCount:  len(answers),
		AuthorityRRs: len(req.Msg.Ns),
		ExtraRRs:     len(req.Msg.Extra),
		AD:           req.Msg.AuthenticatedData,
		DO:           req.Msg.Security,
		Answers:      answerStrings,
	}

	return diagnostic, nil
}
