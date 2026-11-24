package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientCRUDRequests(t *testing.T) {
	var sawAPIKey string
	var sawPaths []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAPIKey = r.Header.Get("api-key")
		sawPaths = append(sawPaths, r.Method+" "+r.URL.RequestURI())
		if r.URL.Path == "/api/groups" && r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(Group{ID: 42, Name: "kids", UseGlobalPolicies: true})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "test-key")
	ctx := context.Background()

	if err := c.AddUpstream(ctx, "1.1.1.1:53"); err != nil {
		t.Fatalf("AddUpstream: %v", err)
	}
	if err := c.DeleteUpstream(ctx, "1.1.1.1:53"); err != nil {
		t.Fatalf("DeleteUpstream: %v", err)
	}
	if err := c.CreateResolution(ctx, Resolution{Domain: "nas.lan", Value: "192.168.1.10", Type: "A"}); err != nil {
		t.Fatalf("CreateResolution: %v", err)
	}
	if err := c.AddWhitelist(ctx, "safe.example.com"); err != nil {
		t.Fatalf("AddWhitelist: %v", err)
	}
	g, err := c.CreateGroup(ctx, Group{Name: "kids", UseGlobalPolicies: true})
	if err != nil || g.ID != 42 {
		t.Fatalf("CreateGroup: err=%v group=%+v", err, g)
	}
	if err := c.DeleteGroup(ctx, 42); err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}

	if sawAPIKey != "test-key" {
		t.Fatalf("expected api-key header test-key, got %q", sawAPIKey)
	}
	if len(sawPaths) != 6 {
		t.Fatalf("expected 6 API calls, got %d: %v", len(sawPaths), sawPaths)
	}
}
