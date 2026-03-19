package database

import (
	"goaway/backend/database"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	require.NoError(t, err)

	err = database.AutoMigrate(db)
	require.NoError(t, err)

	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func sqliteIndexExists(t *testing.T, db *gorm.DB, indexName string) bool {
	t.Helper()
	var count int64
	err := db.Raw("SELECT COUNT(1) FROM sqlite_master WHERE type='index' AND name = ?", indexName).Scan(&count).Error
	require.NoError(t, err)
	return count > 0
}

func TestRunMigrationsIsIdempotent(t *testing.T) {
	db := setupMigrationTestDB(t)

	require.NoError(t, database.RunMigrations(db))
	require.NoError(t, database.RunMigrations(db))

	status, err := database.ListMigrationStatus(db)
	require.NoError(t, err)
	require.NotEmpty(t, status)

	for _, migration := range status {
		assert.True(t, migration.Applied, "migration should be applied: %s", migration.ID)
		require.NotNil(t, migration.AppliedAt)
	}
}

func TestRollbackMigrationsRemovesLatestIndexes(t *testing.T) {
	db := setupMigrationTestDB(t)

	require.NoError(t, database.RunMigrations(db))
	require.True(t, sqliteIndexExists(t, db, "idx_request_logs_dnssec_status"))

	require.NoError(t, database.RollbackMigrations(db, 1))

	assert.False(t, sqliteIndexExists(t, db, "idx_request_logs_dnssec_status"))
	assert.True(t, sqliteIndexExists(t, db, "idx_static_dhcp_leases_mac"))
	assert.True(t, sqliteIndexExists(t, db, "idx_static_dhcp_leases_ip"))
}

func TestRollbackMigrationsRejectsInvalidSteps(t *testing.T) {
	db := setupMigrationTestDB(t)

	err := database.RollbackMigrations(db, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "steps must be greater than zero")
}
