package server

import (
	"errors"
	"goaway/backend/settings"
	"testing"

	"github.com/miekg/dns"
)

func newDNSSECTestServer(mode string, enabled bool) *DNSServer {
	return &DNSServer{
		Config: &settings.Config{
			DNS: settings.DNSConfig{
				DNSSEC: settings.DNSSECConfig{
					Enabled: enabled,
					Mode:    mode,
				},
			},
		},
	}
}

func TestClassifyDNSSECResponse(t *testing.T) {
	t.Run("disabled mode", func(t *testing.T) {
		s := newDNSSECTestServer("off", false)
		msg := &dns.Msg{}
		msg.AuthenticatedData = true
		status := s.classifyDNSSECResponse(msg, nil)
		if status != DNSSECStatusDisabled {
			t.Fatalf("expected disabled, got %s", status)
		}
	})

	t.Run("permissive insecure", func(t *testing.T) {
		s := newDNSSECTestServer("permissive", true)
		status := s.classifyDNSSECResponse(&dns.Msg{}, nil)
		if status != DNSSECStatusInsecure {
			t.Fatalf("expected insecure, got %s", status)
		}
	})

	t.Run("strict bogus", func(t *testing.T) {
		s := newDNSSECTestServer("strict", true)
		status := s.classifyDNSSECResponse(&dns.Msg{}, nil)
		if status != DNSSECStatusBogus {
			t.Fatalf("expected bogus, got %s", status)
		}
	})

	t.Run("strict secure", func(t *testing.T) {
		s := newDNSSECTestServer("strict", true)
		msg := &dns.Msg{}
		msg.AuthenticatedData = true
		status := s.classifyDNSSECResponse(msg, nil)
		if status != DNSSECStatusSecure {
			t.Fatalf("expected secure, got %s", status)
		}
	})

	t.Run("strict on transport error", func(t *testing.T) {
		s := newDNSSECTestServer("strict", true)
		status := s.classifyDNSSECResponse(nil, errors.New("timeout"))
		if status != DNSSECStatusBogus {
			t.Fatalf("expected bogus on error, got %s", status)
		}
	})
}
