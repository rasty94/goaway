package server

import (
	"encoding/csv"
	"encoding/json"
	model "goaway/backend/dns/server/models"
	"goaway/backend/settings"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleLogEntry() model.RequestLogEntry {
	return model.RequestLogEntry{
		Timestamp:    time.Date(2026, 11, 24, 19, 0, 0, 0, time.UTC),
		ClientInfo:   &model.Client{IP: netip.MustParseAddr("192.168.1.10"), Name: "laptop"},
		Domain:       "example.com.",
		QueryType:    "A",
		Status:       "NOERROR",
		DNSSECStatus: DNSSECStatusSecure,
		Protocol:     model.DoQ,
		Blocked:      false,
		Cached:       true,
		ResponseTime: 2500 * time.Microsecond,
	}
}

func TestWriteQueryLogExportJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queries.ndjson")
	cfg := settings.QueryLogExportConfig{
		Enabled: true,
		Format:  "json",
		Target:  path,
	}

	require.NoError(t, writeQueryLogExport(cfg, []model.RequestLogEntry{sampleLogEntry()}))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	require.Len(t, lines, 1)

	var got exportedQueryLog
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &got))
	assert.Equal(t, "192.168.1.10", got.ClientIP)
	assert.Equal(t, "example.com.", got.Domain)
	assert.Equal(t, "DoQ", got.Protocol)
	assert.InDelta(t, 2.5, got.ResponseTimeMs, 0.001)
}

func TestWriteQueryLogExportCSVWritesHeaderOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queries.csv")
	cfg := settings.QueryLogExportConfig{
		Enabled: true,
		Format:  "csv",
		Target:  path,
	}

	require.NoError(t, writeQueryLogExport(cfg, []model.RequestLogEntry{sampleLogEntry()}))
	require.NoError(t, writeQueryLogExport(cfg, []model.RequestLogEntry{sampleLogEntry()}))

	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	rows, err := csv.NewReader(f).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 3, "expected 1 header row + 2 data rows")
	assert.Equal(t, "timestamp", rows[0][0])
	assert.Equal(t, "example.com.", rows[1][3])
	assert.Equal(t, "example.com.", rows[2][3])
}
