package settings

import (
	"path/filepath"
	"testing"
)

// QueryUpstream only iterates DNS.Upstream.Servers. The legacy Preferred and
// Fallback fields are translated into Servers by ApplySchemaUpgrades, but only
// for schemaVersion 1, and a fresh config is written at CurrentSchemaVersion.
// So if the defaults ever populate the legacy fields again, every query on a
// new install answers SERVFAIL with nothing in the logs to explain it.
func TestCreateDefaultSettingsHasUsableUpstreams(t *testing.T) {
	config, err := createDefaultSettings(filepath.Join(t.TempDir(), "settings.yaml"))
	if err != nil {
		t.Fatalf("createDefaultSettings() error = %v", err)
	}

	upstream := config.DNS.Upstream

	if upstream.Preferred != "" || len(upstream.Fallback) != 0 {
		t.Errorf("defaults set the legacy Preferred/Fallback fields (%q, %v); populate Servers instead",
			upstream.Preferred, upstream.Fallback)
	}

	var enabled int
	for _, server := range upstream.Servers {
		if !server.Enabled {
			continue
		}
		enabled++
		if server.Address == "" {
			t.Errorf("upstream %q is enabled with an empty address", server.Name)
		}
		if server.Protocol == "" {
			t.Errorf("upstream %q is enabled with an empty protocol", server.Name)
		}
	}

	if enabled == 0 {
		t.Fatalf("defaults produced no enabled upstream servers, so every query would SERVFAIL")
	}
}
