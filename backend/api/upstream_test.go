package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"goaway/backend/settings"

	"github.com/gin-gonic/gin"
)

func TestCreateUpstreamWritesServers(t *testing.T) {
	withTempConfigDir(t)

	api, c, w := setupHAContext(t, http.MethodPost, map[string]any{
		"upstream": "9.9.9.9",
	})

	api.createUpstream(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	servers := api.Config.DNS.Upstream.Servers
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0].Address != "9.9.9.9:53" {
		t.Fatalf("expected address 9.9.9.9:53, got %s", servers[0].Address)
	}
	if servers[0].Protocol != "udp" {
		t.Fatalf("expected protocol udp, got %s", servers[0].Protocol)
	}
	if !servers[0].Enabled {
		t.Fatalf("expected server to be enabled")
	}
	if len(api.Config.DNS.Upstream.Fallback) != 0 || api.Config.DNS.Upstream.Preferred != "" {
		t.Fatalf("expected legacy fields to stay empty, got fallback=%v preferred=%q",
			api.Config.DNS.Upstream.Fallback, api.Config.DNS.Upstream.Preferred)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/upstream", bytes.NewBuffer(mustJSON(t, map[string]any{
		"upstream": "9.9.9.9",
	})))
	req.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = req

	api.createUpstream(c2)

	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected duplicate upstream to be rejected with 400, got %d", w2.Code)
	}
}

func TestUpdatePreferredMovesServerToFront(t *testing.T) {
	withTempConfigDir(t)

	api, c, w := setupHAContext(t, http.MethodPut, map[string]any{
		"upstream": "8.8.8.8:53",
	})
	api.Config.DNS.Upstream.Servers = []settings.UpstreamServer{
		{Name: "1.1.1.1", Address: "1.1.1.1:53", Protocol: "udp", Enabled: true},
		{Name: "8.8.8.8", Address: "8.8.8.8:53", Protocol: "udp", Enabled: true},
	}

	api.updatePreferredUpstream(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	servers := api.Config.DNS.Upstream.Servers
	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}
	if servers[0].Address != "8.8.8.8:53" {
		t.Fatalf("expected 8.8.8.8:53 to be first, got %s", servers[0].Address)
	}
}

func TestDeleteUpstreamRemovesServer(t *testing.T) {
	withTempConfigDir(t)

	api := &API{Config: &settings.Config{}}
	api.Config.DNS.Upstream.Servers = []settings.UpstreamServer{
		{Name: "1.1.1.1", Address: "1.1.1.1:53", Protocol: "udp", Enabled: true},
		{Name: "8.8.8.8", Address: "8.8.8.8:53", Protocol: "udp", Enabled: true},
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/upstream?upstream=1.1.1.1:53", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.deleteUpstream(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if len(api.Config.DNS.Upstream.Servers) != 1 {
		t.Fatalf("expected 1 server remaining, got %d", len(api.Config.DNS.Upstream.Servers))
	}
	if api.Config.DNS.Upstream.Servers[0].Address != "8.8.8.8:53" {
		t.Fatalf("expected 8.8.8.8:53 to remain, got %s", api.Config.DNS.Upstream.Servers[0].Address)
	}

	reqMissing := httptest.NewRequest(http.MethodDelete, "/api/upstream?upstream=9.9.9.9:53", nil)
	wMissing := httptest.NewRecorder()
	cMissing, _ := gin.CreateTestContext(wMissing)
	cMissing.Request = reqMissing

	api.deleteUpstream(cMissing)

	if wMissing.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 for unknown upstream, got %d", wMissing.Code)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	return b
}
