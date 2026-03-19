package settings

import (
	"fmt"
	"strings"
)

const CurrentSchemaVersion = 1

func (config *Config) ApplySchemaUpgrades() (bool, error) {
	changed := false

	if config.SchemaVersion < 0 {
		return false, fmt.Errorf("invalid schemaVersion: %d", config.SchemaVersion)
	}

	if config.SchemaVersion == 0 {
		config.SchemaVersion = 1
		changed = true
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

	return changed, nil
}
