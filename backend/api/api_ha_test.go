package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"goaway/backend/cluster"
	"goaway/backend/settings"

	"github.com/gin-gonic/gin"
)

func setupHAContext(t *testing.T, method string, body any) (*API, *gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	payload := []byte{}
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body failed: %v", err)
		}
	}

	req := httptest.NewRequest(method, "/api/ha/config", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api := &API{Config: &settings.Config{}}
	return api, c, w
}

func withTempConfigDir(t *testing.T) {
	t.Helper()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	if err := os.MkdirAll("config", 0755); err != nil {
		t.Fatalf("mkdir config failed: %v", err)
	}
}

func TestSaveHAConfigRejectsInvalidMode(t *testing.T) {
	withTempConfigDir(t)

	api, c, w := setupHAContext(t, http.MethodPost, map[string]any{
		"enabled": true,
		"mode":    "active",
	})

	api.saveHAConfig(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestSaveHAConfigRejectsInvalidProviderInReplica(t *testing.T) {
	withTempConfigDir(t)

	api, c, w := setupHAContext(t, http.MethodPost, map[string]any{
		"enabled":               true,
		"mode":                  "replica",
		"primaryBackupProvider": "ftp",
	})

	api.saveHAConfig(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestSaveHAConfigSuccessSetsDefaults(t *testing.T) {
	withTempConfigDir(t)

	api, c, w := setupHAContext(t, http.MethodPost, map[string]any{
		"enabled":               true,
		"mode":                  "replica",
		"primaryBackupProvider": "s3",
		"primaryBackupBucket":   "bucket",
	})

	api.saveHAConfig(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if api.Config.HighAvailability.ReplicaSyncInterval != "15m" {
		t.Fatalf("expected default interval 15m, got %s", api.Config.HighAvailability.ReplicaSyncInterval)
	}
}

func TestTriggerHASyncRequiresReplicaMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	api := &API{Config: &settings.Config{HighAvailability: settings.HighAvailabilityConfig{Enabled: true, Mode: "primary"}}}

	req := httptest.NewRequest(http.MethodPost, "/api/ha/sync-now", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.triggerHASync(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestTriggerHASyncRequiresManager(t *testing.T) {
	gin.SetMode(gin.TestMode)
	api := &API{Config: &settings.Config{HighAvailability: settings.HighAvailabilityConfig{Enabled: true, Mode: "replica"}}}

	req := httptest.NewRequest(http.MethodPost, "/api/ha/sync-now", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.triggerHASync(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
}

func newClusterRouter(secret string) (*API, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	api := &API{Config: &settings.Config{}}
	api.Config.HighAvailability.Secret = secret
	r := gin.New()
	r.POST("/api/native/cluster/heartbeat", api.clusterAuth(), api.handleHeartbeat)
	return api, r
}

func postHeartbeat(r *gin.Engine, sign func(*http.Request, []byte)) *httptest.ResponseRecorder {
	body := []byte(`{"sourceId":"peer-1","role":"replica","priority":1}`)
	req := httptest.NewRequest(http.MethodPost, "/api/native/cluster/heartbeat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if sign != nil {
		sign(req, body)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestClusterAuthRejectsUnsignedRequest(t *testing.T) {
	_, r := newClusterRouter("s3cret")
	if w := postHeartbeat(r, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestClusterAuthRejectsWrongSecret(t *testing.T) {
	_, r := newClusterRouter("s3cret")
	w := postHeartbeat(r, func(req *http.Request, body []byte) { cluster.SignRequest(req, "wrong", body) })
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestClusterAuthFailsClosedWithoutSecret(t *testing.T) {
	_, r := newClusterRouter("")
	w := postHeartbeat(r, func(req *http.Request, body []byte) { cluster.SignRequest(req, "", body) })
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestClusterAuthAcceptsSignedRequest(t *testing.T) {
	_, r := newClusterRouter("s3cret")
	w := postHeartbeat(r, func(req *http.Request, body []byte) { cluster.SignRequest(req, "s3cret", body) })
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp cluster.HeartbeatResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.Status != "online" {
		t.Fatalf("unexpected heartbeat response: %s", w.Body.String())
	}
}

func TestAuditThrottleAllowsOnePerWindow(t *testing.T) {
	var throttle auditThrottle
	if !throttle.allow() {
		t.Fatal("first rejection should be audited")
	}
	if throttle.allow() {
		t.Fatal("second rejection inside the window should not be audited")
	}
	throttle.last = time.Now().Add(-2 * clusterAuditWindow)
	if !throttle.allow() {
		t.Fatal("rejection after the window should be audited again")
	}
}
