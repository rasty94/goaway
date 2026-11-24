package server

import (
	"goaway/backend/settings"
	"testing"

	"codeberg.org/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttachEDEWiredIntoOPTRecord(t *testing.T) {
	srv := &DNSServer{
		Config: &settings.Config{
			DNS: settings.DNSConfig{EDEEnabled: true},
		},
	}

	msg := dns.NewMsg("ads.example.com.", dns.TypeA)
	msg.Response = true
	srv.attachEDE(msg, dns.ExtendedErrorFiltered, blockedEDEText("ads"))

	require.NoError(t, msg.Pack())

	unpacked := new(dns.Msg)
	unpacked.Data = msg.Data
	require.NoError(t, unpacked.Unpack())

	var found *dns.EDE
	for _, rr := range unpacked.Pseudo {
		if ede, ok := rr.(*dns.EDE); ok {
			found = ede
			break
		}
	}

	require.NotNil(t, found, "expected EDE pseudo-RR in unpacked OPT section")
	assert.Equal(t, dns.ExtendedErrorFiltered, found.InfoCode)
	assert.Equal(t, "blocked by goaway (category: ads)", found.ExtraText)
}

func TestAttachEDEDisabledByDefault(t *testing.T) {
	srv := &DNSServer{
		Config: &settings.Config{
			DNS: settings.DNSConfig{EDEEnabled: false},
		},
	}

	msg := dns.NewMsg("ads.example.com.", dns.TypeA)
	srv.attachEDE(msg, dns.ExtendedErrorFiltered, blockedEDEText(""))
	assert.Empty(t, msg.Pseudo)
}

func TestAttachEDEDNSBogusAndStale(t *testing.T) {
	srv := &DNSServer{
		Config: &settings.Config{
			DNS: settings.DNSConfig{EDEEnabled: true},
		},
	}

	bogusMsg := dns.NewMsg("bad.dnssec.test.", dns.TypeA)
	srv.attachEDE(bogusMsg, dns.ExtendedErrorDNSBogus, "DNSSEC validation failed (strict mode)")
	require.Len(t, bogusMsg.Pseudo, 1)
	ede, ok := bogusMsg.Pseudo[0].(*dns.EDE)
	require.True(t, ok)
	assert.Equal(t, dns.ExtendedErrorDNSBogus, ede.InfoCode)

	staleMsg := dns.NewMsg("stale.example.com.", dns.TypeA)
	srv.attachEDE(staleMsg, dns.ExtendedErrorStaleAnswer, "served from stale cache")
	require.Len(t, staleMsg.Pseudo, 1)
	ede, ok = staleMsg.Pseudo[0].(*dns.EDE)
	require.True(t, ok)
	assert.Equal(t, dns.ExtendedErrorStaleAnswer, ede.InfoCode)
}
