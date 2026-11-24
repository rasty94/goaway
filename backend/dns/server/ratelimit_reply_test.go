package server

import (
	"bytes"
	model "goaway/backend/dns/server/models"
	"net"
	"net/netip"
	"testing"

	"codeberg.org/miekg/dns"
)

// captureWriter records what is written; other ResponseWriter methods are unused here.
type captureWriter struct {
	dns.ResponseWriter
	buf bytes.Buffer
}

func (w *captureWriter) Write(p []byte) (int, error) { return w.buf.Write(p) }

// Conn returns nil (not a UDP socket), so dns.Msg.WriteTo uses TCP framing: 2-byte length prefix.
func (w *captureWriter) Conn() net.Conn { return nil }

func TestWriteRateLimitedResponseRefusesWithoutMutatingRequest(t *testing.T) {
	req := dns.NewMsg("example.com.", dns.TypeA)
	req.ID = 4242
	w := &captureWriter{}
	s := &DNSServer{}

	entry := s.writeRateLimitedResponse(&Request{
		ResponseWriter: w,
		Msg:            req,
		Question:       req.Question[0],
		Client:         &model.Client{IP: netip.MustParseAddr("192.0.2.1")},
		Protocol:       model.UDP,
	}, 0)

	if req.Response || req.Rcode != dns.RcodeSuccess {
		t.Fatalf("request was mutated: response=%v rcode=%d", req.Response, req.Rcode)
	}

	out := w.buf.Bytes()
	if len(out) < 2 {
		t.Fatalf("no reply written")
	}
	reply := &dns.Msg{Data: out[2:]}
	if err := reply.Unpack(); err != nil {
		t.Fatalf("unpack reply: %v", err)
	}
	if !reply.Response || reply.Rcode != dns.RcodeRefused || reply.ID != 4242 {
		t.Fatalf("bad reply: response=%v rcode=%d id=%d", reply.Response, reply.Rcode, reply.ID)
	}
	if len(reply.Question) != 1 || reply.Question[0].Header().Name != "example.com." {
		t.Fatalf("question not preserved: %v", reply.Question)
	}
	if entry.Domain != "example.com." {
		t.Fatalf("log entry domain = %q", entry.Domain)
	}
}
