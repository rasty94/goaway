package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

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
