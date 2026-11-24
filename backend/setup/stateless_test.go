package setup

import (
	"goaway/backend/settings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateConfigStatelessFlag(t *testing.T) {
	cfg := &settings.Config{
		API: settings.APIConfig{ReadOnly: false},
		Misc: settings.MiscConfig{
			Dashboard:   true,
			InAppUpdate: true,
		},
	}

	stateless := true
	UpdateConfig(cfg, &SetFlags{Stateless: &stateless})

	assert.True(t, cfg.Misc.Stateless)
	assert.False(t, cfg.Misc.Dashboard)
	assert.True(t, cfg.API.ReadOnly)
	assert.False(t, cfg.Misc.InAppUpdate)
}

func TestUpdateConfigStatelessEnvVar(t *testing.T) {
	t.Setenv("GOAWAY_STATELESS", "true")

	cfg := &settings.Config{
		API: settings.APIConfig{ReadOnly: false},
		Misc: settings.MiscConfig{
			Dashboard:   true,
			InAppUpdate: true,
		},
	}

	UpdateConfig(cfg, &SetFlags{})

	assert.True(t, cfg.Misc.Stateless)
	assert.False(t, cfg.Misc.Dashboard)
	assert.True(t, cfg.API.ReadOnly)
	assert.False(t, cfg.Misc.InAppUpdate)
}
