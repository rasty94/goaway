package database

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func Initialize() *gorm.DB {
	db, err := InitializeWithOptions(false)
	if err != nil {
		log.Fatal(err)
	}
	return db
}

// InitializeWithOptions opens either the persistent SQLite database under data/database.db
// or, when inMemory is true (stateless profile), an in-memory shared-cache SQLite database
// without creating any files on disk.
func InitializeWithOptions(inMemory bool) (*gorm.DB, error) {
	var dsn string
	if inMemory {
		dsn = "file:goaway_stateless?mode=memory&cache=shared&_busy_timeout=5000"
	} else {
		if err := os.MkdirAll("data", 0750); err != nil {
			return nil, fmt.Errorf("failed to create data directory: %w", err)
		}
		databasePath := filepath.Join("data", "database.db")
		dsn = fmt.Sprintf("file:%s?cache=shared&mode=rwc&_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_query_only=false", databasePath)
	}

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		TranslateError: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed while initializing database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}
	if inMemory {
		// Keep a single persistent connection so the in-memory database is never reclaimed.
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
	} else {
		sqlDB.SetMaxOpenConns(25)
		sqlDB.SetMaxIdleConns(5)
	}
	sqlDB.SetConnMaxLifetime(0)

	if err := AutoMigrate(db); err != nil {
		return nil, fmt.Errorf("auto migrate failed: %w", err)
	}

	if err := RunMigrations(db); err != nil {
		return nil, fmt.Errorf("migration runner failed: %w", err)
	}

	return db, nil
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&Source{},
		&Blacklist{},
		&Whitelist{},
		&RequestLog{},
		&RequestLogIP{},
		&MacAddress{},
		&User{},
		&APIKey{},
		&Notification{},
		&Prefetch{},
		&Audit{},
		&Alert{},
		&ClientGroup{},
		&ClientGroupAssignment{},
		&GroupBlockedDomain{},
		&GroupAllowedDomain{},
		&StaticDHCPLease{},
		&ActiveDHCPLease{},
		&StaticDHCPv6Lease{},
		&ActiveDHCPv6Lease{},
		&Policy{},
		&Schedule{},
		&PolicyAssignment{},
		&PolicyCategory{},
		&Resolution{},
		&ReplicationOutbox{},
		&WebAuthnCredential{},
		&UserTOTP{},
	)
}
