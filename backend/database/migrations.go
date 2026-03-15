package database

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

type dbMigration struct {
	ID          string
	Description string
	Up          func(tx *gorm.DB) error
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
	},
}

func RunMigrations(db *gorm.DB) error {
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

func migrationApplied(db *gorm.DB, id string) (bool, error) {
	var count int64
	if err := db.Model(&schemaMigration{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check migration %s: %w", id, err)
	}
	return count > 0, nil
}
