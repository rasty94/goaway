package server

import (
	"fmt"
	model "goaway/backend/dns/server/models"
	"strings"
	"time"

	"github.com/miekg/dns"
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

	msg := &dns.Msg{}
	msg.SetQuestion(dns.Fqdn(strings.TrimSpace(domain)), qtype)

	req := &Request{
		Sent:     time.Now(),
		Msg:      msg,
		Question: msg.Question[0],
		Client:   &model.Client{IP: "diagnostic", Name: "diagnostic"},
		Protocol: model.UDP,
	}

	answers, _, status, dnssecStatus := s.QueryUpstream(req)

	answerStrings := make([]string, 0, len(answers))
	for _, rr := range answers {
		answerStrings = append(answerStrings, rr.String())
	}

	do := false
	if req.Msg != nil {
		if opt := req.Msg.IsEdns0(); opt != nil {
			do = opt.Do()
		}
	}

	diagnostic := &DNSSECDiagnostic{
		Domain:       dns.Fqdn(strings.TrimSpace(domain)),
		Type:         dns.TypeToString[qtype],
		Status:       status,
		DNSSECStatus: dnssecStatus,
		AnswerCount:  len(answers),
		AuthorityRRs: len(req.Msg.Ns),
		ExtraRRs:     len(req.Msg.Extra),
		AD:           req.Msg.AuthenticatedData,
		DO:           do,
		Answers:      answerStrings,
	}

	return diagnostic, nil
}
