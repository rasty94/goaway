package cluster

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSignRequestRoundTrip(t *testing.T) {
	body := []byte(`{"type":"blacklist.add"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/native/cluster/replicate", strings.NewReader(string(body)))
	SignRequest(req, "s3cret", body)

	path := "/api/native/cluster/replicate"
	if err := VerifyRequest(req.Header, "s3cret", path, body, time.Now()); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	if err := VerifyRequest(req.Header, "other", path, body, time.Now()); err == nil {
		t.Fatal("wrong secret accepted")
	}
	if err := VerifyRequest(req.Header, "s3cret", path, []byte(`{"type":"blacklist.remove"}`), time.Now()); err == nil {
		t.Fatal("tampered body accepted")
	}
	if err := VerifyRequest(req.Header, "s3cret", "/api/native/cluster/heartbeat", body, time.Now()); err == nil {
		t.Fatal("signature accepted for another endpoint")
	}
	if err := VerifyRequest(req.Header, "s3cret", path, body, time.Now().Add(2*SignatureWindow)); err == nil {
		t.Fatal("stale timestamp accepted")
	}
	if err := VerifyRequest(http.Header{}, "s3cret", path, body, time.Now()); err == nil {
		t.Fatal("missing headers accepted")
	}
	if err := VerifyRequest(req.Header, "", path, body, time.Now()); err == nil {
		t.Fatal("empty secret accepted")
	}
}
