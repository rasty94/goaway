package database

import (
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"
)

type dbMigration struct {
	ID          string
	Description string
	Up          func(tx *gorm.DB) error
	Down        func(tx *gorm.DB) error
}

type schemaMigration struct {
	ID        string    `gorm:"primaryKey;size:32"`
	AppliedAt time.Time `gorm:"not null"`
}

var migrations = []dbMigration{
	{
		ID:          "000001_bootstrap_schema_migrations",
		Description: "Create internal schema migration tracking table",
		Up: func(tx *gorm.DB) error {
			// Table creation is handled by AutoMigrate below.
			return nil
		},
		Down: func(tx *gorm.DB) error {
			// Keep bootstrap migration non-destructive during rollback.
			return nil
		},
	},
	{
		ID:          "000002_request_log_indexes",
		Description: "Add explicit indexes for request log performance",
		Up: func(tx *gorm.DB) error {
			// Keep SQL idempotent so reruns are safe during bootstrap.
			if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_request_logs_timestamp ON request_logs(timestamp)").Error; err != nil {
				return err
			}
			if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_request_logs_client_ip ON request_logs(client_ip)").Error; err != nil {
				return err
			}
			if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_request_logs_domain ON request_logs(domain)").Error; err != nil {
				return err
			}
			return nil
		},
		Down: func(tx *gorm.DB) error {
			if err := tx.Exec("DROP INDEX IF EXISTS idx_request_logs_timestamp").Error; err != nil {
				return err
			}
			if err := tx.Exec("DROP INDEX IF EXISTS idx_request_logs_client_ip").Error; err != nil {
				return err
			}
			if err := tx.Exec("DROP INDEX IF EXISTS idx_request_logs_domain").Error; err != nil {
				return err
			}
			return nil
		},
	},
	{
		ID:          "000003_group_management_indexes",
		Description: "Add explicit indexes for group management tables",
		Up: func(tx *gorm.DB) error {
			if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_client_group_assignment_lookup ON client_group_assignments(identifier_type, identifier)").Error; err != nil {
				return err
			}
			if err := tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_client_groups_default_unique ON client_groups(is_default) WHERE is_default = 1").Error; err != nil {
				return err
			}
			if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_group_blocked_domains_group_id ON group_blocked_domains(group_id)").Error; err != nil {
				return err
			}
			if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_group_allowed_domains_group_id ON group_allowed_domains(group_id)").Error; err != nil {
				return err
			}
			return nil
		},
		Down: func(tx *gorm.DB) error {
			if err := tx.Exec("DROP INDEX IF EXISTS idx_client_group_assignment_lookup").Error; err != nil {
				return err
			}
			if err := tx.Exec("DROP INDEX IF EXISTS idx_client_groups_default_unique").Error; err != nil {
				return err
			}
			if err := tx.Exec("DROP INDEX IF EXISTS idx_group_blocked_domains_group_id").Error; err != nil {
				return err
			}
			if err := tx.Exec("DROP INDEX IF EXISTS idx_group_allowed_domains_group_id").Error; err != nil {
				return err
			}
			return nil
		},
	},
	{
		ID:          "000004_static_dhcp_lease_indexes",
		Description: "Add explicit indexes for static DHCP leases",
		Up: func(tx *gorm.DB) error {
			if err := tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_static_dhcp_leases_mac ON static_dhcp_leases(mac)").Error; err != nil {
				return err
			}
			if err := tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_static_dhcp_leases_ip ON static_dhcp_leases(ip)").Error; err != nil {
				return err
			}
			return nil
		},
		Down: func(tx *gorm.DB) error {
			if err := tx.Exec("DROP INDEX IF EXISTS idx_static_dhcp_leases_mac").Error; err != nil {
				return err
			}
			if err := tx.Exec("DROP INDEX IF EXISTS idx_static_dhcp_leases_ip").Error; err != nil {
				return err
			}
			return nil
		},
	},
	{
		ID:          "000005_dnssec_status_index",
		Description: "Add explicit index for DNSSEC status filtering in request logs",
		Up: func(tx *gorm.DB) error {
			if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_request_logs_dnssec_status ON request_logs(dnssec_status)").Error; err != nil {
				return err
			}
			return nil
		},
		Down: func(tx *gorm.DB) error {
			if err := tx.Exec("DROP INDEX IF EXISTS idx_request_logs_dnssec_status").Error; err != nil {
				return err
			}
			return nil
		},
	},
}

type MigrationStatus struct {
	ID          string
	Description string
	Applied     bool
	AppliedAt   *time.Time
}

func RunMigrations(db *gorm.DB) error {
	if err := validateMigrationPlan(migrations); err != nil {
		return err
	}

	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		return fmt.Errorf("failed to initialize migration table: %w", err)
	}

	for _, migration := range migrations {
		alreadyApplied, err := migrationApplied(db, migration.ID)
		if err != nil {
			return err
		}
		if alreadyApplied {
			continue
		}

		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := migration.Up(tx); err != nil {
				return fmt.Errorf("migration %s failed: %w", migration.ID, err)
			}

			record := schemaMigration{
				ID:        migration.ID,
				AppliedAt: time.Now().UTC(),
			}
			if err := tx.Create(&record).Error; err != nil {
				return fmt.Errorf("failed to persist migration %s: %w", migration.ID, err)
			}
			return nil
		}); err != nil {
			return err
		}
	}

	return nil
}

func RollbackMigrations(db *gorm.DB, steps int) error {
	if steps <= 0 {
		return fmt.Errorf("steps must be greater than zero")
	}

	if err := validateMigrationPlan(migrations); err != nil {
		return err
	}

	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		return fmt.Errorf("failed to initialize migration table: %w", err)
	}

	applied, err := appliedMigrations(db)
	if err != nil {
		return err
	}
	if len(applied) == 0 {
		return nil
	}

	rollbackCount := steps
	if rollbackCount > len(applied) {
		rollbackCount = len(applied)
	}

	for i := len(applied) - 1; i >= len(applied)-rollbackCount; i-- {
		record := applied[i]
		migration, ok := findMigrationByID(record.ID)
		if !ok {
			return fmt.Errorf("cannot rollback unknown migration %s", record.ID)
		}

		if err := db.Transaction(func(tx *gorm.DB) error {
			if migration.Down != nil {
				if err := migration.Down(tx); err != nil {
					return fmt.Errorf("rollback %s failed: %w", migration.ID, err)
				}
			}

			if err := tx.Where("id = ?", migration.ID).Delete(&schemaMigration{}).Error; err != nil {
				return fmt.Errorf("failed to delete migration record %s: %w", migration.ID, err)
			}

			return nil
		}); err != nil {
			return err
		}
	}

	return nil
}

func ListMigrationStatus(db *gorm.DB) ([]MigrationStatus, error) {
	if err := validateMigrationPlan(migrations); err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		return nil, fmt.Errorf("failed to initialize migration table: %w", err)
	}

	applied, err := appliedMigrations(db)
	if err != nil {
		return nil, err
	}

	appliedMap := make(map[string]schemaMigration, len(applied))
	for _, record := range applied {
		appliedMap[record.ID] = record
	}

	status := make([]MigrationStatus, 0, len(migrations))
	for _, migration := range migrations {
		item := MigrationStatus{
			ID:          migration.ID,
			Description: migration.Description,
			Applied:     false,
		}

		if record, ok := appliedMap[migration.ID]; ok {
			appliedAt := record.AppliedAt
			item.Applied = true
			item.AppliedAt = &appliedAt
		}

		status = append(status, item)
	}

	return status, nil
}

func migrationApplied(db *gorm.DB, id string) (bool, error) {
	var count int64
	if err := db.Model(&schemaMigration{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check migration %s: %w", id, err)
	}
	return count > 0, nil
}

func appliedMigrations(db *gorm.DB) ([]schemaMigration, error) {
	var records []schemaMigration
	if err := db.Order("id ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch applied migrations: %w", err)
	}
	return records, nil
}

func findMigrationByID(id string) (dbMigration, bool) {
	for _, migration := range migrations {
		if migration.ID == id {
			return migration, true
		}
	}

	return dbMigration{}, false
}

func validateMigrationPlan(plan []dbMigration) error {
	if len(plan) == 0 {
		return fmt.Errorf("migration plan is empty")
	}

	seen := make(map[string]struct{}, len(plan))
	ids := make([]string, 0, len(plan))

	for _, migration := range plan {
		if migration.ID == "" {
			return fmt.Errorf("migration id cannot be empty")
		}
		if migration.Up == nil {
			return fmt.Errorf("migration %s does not define an up function", migration.ID)
		}
		if _, exists := seen[migration.ID]; exists {
			return fmt.Errorf("duplicate migration id: %s", migration.ID)
		}

		seen[migration.ID] = struct{}{}
		ids = append(ids, migration.ID)
	}

	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)
	for i := range sorted {
		if sorted[i] != ids[i] {
			return fmt.Errorf("migrations must be ordered by id ascending")
		}
	}

	return nil
}
