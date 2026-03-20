package settings

import (
	"fmt"
	"strings"
)

const CurrentSchemaVersion = 2

func (config *Config) ApplySchemaUpgrades() (bool, error) {
	changed := false

	if config.SchemaVersion < 0 {
		return false, fmt.Errorf("invalid schemaVersion: %d", config.SchemaVersion)
	}

	if config.SchemaVersion == 0 {
		config.SchemaVersion = 1
		changed = true
	}

	if config.SchemaVersion == 1 {
		// Migrate UpstreamConfig to Servers
		if len(config.DNS.Upstream.Servers) == 0 {
			if config.DNS.Upstream.Preferred != "" {
				config.DNS.Upstream.Servers = append(config.DNS.Upstream.Servers, UpstreamServer{
					Name:     "Preferred (Migrated)",
					Address:  config.DNS.Upstream.Preferred,
					Protocol: "udp",
					Enabled:  true,
				})
			}
			for i, f := range config.DNS.Upstream.Fallback {
				if f != "" {
					config.DNS.Upstream.Servers = append(config.DNS.Upstream.Servers, UpstreamServer{
						Name:     fmt.Sprintf("Fallback %d (Migrated)", i+1),
						Address:  f,
						Protocol: "udp",
						Enabled:  true,
					})
				}
			}
			config.DNS.Upstream.Preferred = ""
			config.DNS.Upstream.Fallback = nil
			config.SchemaVersion = 2
			changed = true
		} else {
			config.SchemaVersion = 2
			changed = true
		}
	}

	if config.SchemaVersion > CurrentSchemaVersion {
		return false, fmt.Errorf("settings schema version %d is newer than supported version %d", config.SchemaVersion, CurrentSchemaVersion)
	}

	if strings.TrimSpace(config.HighAvailability.Mode) == "" {
		config.HighAvailability.Mode = "primary"
		changed = true
	}

	if strings.TrimSpace(config.HighAvailability.ReplicaSyncInterval) == "" {
		config.HighAvailability.ReplicaSyncInterval = "15m"
		changed = true
	}

	if strings.TrimSpace(config.RemoteBackup.Schedule) == "" {
		config.RemoteBackup.Schedule = "manual"
		changed = true
	}

	mode := strings.ToLower(strings.TrimSpace(config.DNS.DNSSEC.Mode))
	switch mode {
	case "", "off":
		if mode != "off" {
			changed = true
		}
		config.DNS.DNSSEC.Mode = "off"
		config.DNS.DNSSEC.Enabled = false
	case "permissive", "strict":
		if config.DNS.DNSSEC.Mode != mode {
			changed = true
		}
		config.DNS.DNSSEC.Mode = mode
		config.DNS.DNSSEC.Enabled = true
	default:
		return false, fmt.Errorf("invalid dnssec mode: %s", config.DNS.DNSSEC.Mode)
	}

	return changed, nil
}
