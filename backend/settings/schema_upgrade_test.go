package settings

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestApplySchemaUpgradesFromLegacyConfig(t *testing.T) {
	cfg := Config{}

	changed, err := cfg.ApplySchemaUpgrades()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatalf("expected config to be upgraded")
	}
	if cfg.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("unexpected schema version: %d", cfg.SchemaVersion)
	}
	if cfg.HighAvailability.Mode != "primary" {
		t.Fatalf("unexpected HA mode: %s", cfg.HighAvailability.Mode)
	}
	if cfg.HighAvailability.ReplicaSyncInterval != "15m" {
		t.Fatalf("unexpected HA interval: %s", cfg.HighAvailability.ReplicaSyncInterval)
	}
	if cfg.RemoteBackup.Schedule != "manual" {
		t.Fatalf("unexpected backup schedule: %s", cfg.RemoteBackup.Schedule)
	}
}

func TestApplySchemaUpgradesRejectsFutureVersion(t *testing.T) {
	cfg := Config{SchemaVersion: CurrentSchemaVersion + 1}

	_, err := cfg.ApplySchemaUpgrades()
	if err == nil {
		t.Fatalf("expected error for future schema version")
	}
	if !strings.Contains(err.Error(), "newer than supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadSettingsPersistsUpgradedSchema(t *testing.T) {
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
		t.Fatalf("mkdir failed: %v", err)
	}

	settingsPath := filepath.Join("config", "settings.yaml")
	if err := os.WriteFile(settingsPath, []byte("api:\n  port: 8080\n"), 0644); err != nil {
		t.Fatalf("write settings failed: %v", err)
	}

	cfg, err := LoadSettings()
	if err != nil {
		t.Fatalf("load settings failed: %v", err)
	}

	if cfg.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("expected schema version %d, got %d", CurrentSchemaVersion, cfg.SchemaVersion)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read upgraded settings failed: %v", err)
	}
	if !strings.Contains(string(data), "schemaVersion: "+strconv.Itoa(CurrentSchemaVersion)) {
		t.Fatalf("expected upgraded settings file to contain schemaVersion")
	}
}
