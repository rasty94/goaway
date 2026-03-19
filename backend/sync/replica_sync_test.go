package sync

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"goaway/backend/settings"
)

type importerStub struct {
	called bool
	size   int64
}

func (s *importerStub) ImportTeleporterData(_ io.Reader, size int64) error {
	s.called = true
	s.size = size
	return nil
}

func withTempWorkingDir(t *testing.T) {
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

func newTestConfig() *settings.Config {
	return &settings.Config{
		HighAvailability: settings.HighAvailabilityConfig{
			Enabled:               true,
			Mode:                  "replica",
			ReplicaSyncInterval:   "1m",
			PrimaryBackupProvider: "local",
		},
	}
}

func TestLatestBackupFromWebDAV(t *testing.T) {
	xmlPayload := []byte(`<?xml version="1.0"?>
<multistatus xmlns="DAV:">
  <response>
    <href>/backups/goaway-backup-older.zip</href>
    <propstat>
      <prop>
        <getcontentlength>10</getcontentlength>
        <getlastmodified>Mon, 18 Mar 2026 10:00:00 GMT</getlastmodified>
      </prop>
    </propstat>
  </response>
  <response>
    <href>/backups/goaway-backup-newer.zip</href>
    <propstat>
      <prop>
        <getcontentlength>20</getcontentlength>
        <getlastmodified>Mon, 18 Mar 2026 11:00:00 GMT</getlastmodified>
      </prop>
    </propstat>
  </response>
</multistatus>`)

	latest, err := latestBackupFromWebDAV(xmlPayload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if latest != "/backups/goaway-backup-newer.zip" {
		t.Fatalf("unexpected latest backup: %s", latest)
	}
}

func TestDownloadFromLocalSelectsLatestFile(t *testing.T) {
	config := newTestConfig()
	stub := &importerStub{}
	rsm := NewReplicaSyncManager(config, stub)

	dir := t.TempDir()
	config.HighAvailability.PrimaryBackupEndpoint = dir

	olderPath := filepath.Join(dir, "goaway-backup-older.zip")
	newerPath := filepath.Join(dir, "goaway-backup-newer.zip")

	if err := os.WriteFile(olderPath, []byte("older"), 0644); err != nil {
		t.Fatalf("write older failed: %v", err)
	}
	if err := os.WriteFile(newerPath, []byte("newer"), 0644); err != nil {
		t.Fatalf("write newer failed: %v", err)
	}

	olderTime := time.Now().Add(-2 * time.Hour)
	newerTime := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(olderPath, olderTime, olderTime); err != nil {
		t.Fatalf("chtimes older failed: %v", err)
	}
	if err := os.Chtimes(newerPath, newerTime, newerTime); err != nil {
		t.Fatalf("chtimes newer failed: %v", err)
	}

	data, err := rsm.downloadFromLocal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "newer" {
		t.Fatalf("expected newest file content, got: %s", string(data))
	}
}

func TestSyncNowUpdatesLastSyncTimeAndImports(t *testing.T) {
	withTempWorkingDir(t)

	config := newTestConfig()
	stub := &importerStub{}
	rsm := NewReplicaSyncManager(config, stub)

	dir := t.TempDir()
	config.HighAvailability.PrimaryBackupEndpoint = dir

	backupPath := filepath.Join(dir, "goaway-backup-now.zip")
	payload := []byte("zip-bytes")
	if err := os.WriteFile(backupPath, payload, 0644); err != nil {
		t.Fatalf("write backup failed: %v", err)
	}

	if err := rsm.SyncNow(); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	if !stub.called {
		t.Fatalf("expected importer to be called")
	}
	if stub.size != int64(len(payload)) {
		t.Fatalf("unexpected importer payload size: %d", stub.size)
	}
	if config.HighAvailability.LastSyncTime.IsZero() {
		t.Fatalf("expected last sync time to be updated")
	}
}
