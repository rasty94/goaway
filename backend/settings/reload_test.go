package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReloadFromFilePreservesSecretsAndUpdatesDNS(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.yaml")

	yamlContent := `
schemaVersion: 2
api:
  port: 8080
  authentication: true
  readOnly: true
dns:
  cacheEnabled: false
  edeEnabled: true
  blockedQueryTypes:
    - HTTPS
    - SVCB
  upstream:
    strategy: parallel_best
    servers:
      - name: cloudflare
        address: 1.1.1.1:53
        protocol: udp
        enabled: true
`
	require.NoError(t, os.WriteFile(path, []byte(yamlContent), 0600))

	cfg := &Config{
		API: APIConfig{JWTSecret: "existing-jwt-secret"},
		HighAvailability: HighAvailabilityConfig{
			Secret: "existing-ha-secret",
		},
	}

	require.NoError(t, cfg.ReloadFromFile(path))

	assert.True(t, cfg.API.ReadOnly)
	assert.Equal(t, "existing-jwt-secret", cfg.API.JWTSecret)
	assert.Equal(t, "existing-ha-secret", cfg.HighAvailability.Secret)

	snap := cfg.DNSSnapshot()
	assert.False(t, snap.CacheEnabled)
	assert.True(t, snap.EDEEnabled)
	assert.Equal(t, "parallel_best", snap.Upstream.Strategy)
	assert.Equal(t, []string{"HTTPS", "SVCB"}, snap.BlockedQueryTypes)
	require.Len(t, snap.Upstream.Servers, 1)
	assert.Equal(t, "1.1.1.1:53", snap.Upstream.Servers[0].Address)
}
