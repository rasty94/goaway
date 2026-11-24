package database

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitializeWithOptionsStatelessInMemory(t *testing.T) {
	tmpDir := t.TempDir()
	origWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	db, err := InitializeWithOptions(true)
	require.NoError(t, err)
	require.NotNil(t, db)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	// Stateless mode must NOT create data/ or database.db on disk.
	_, statErr := os.Stat(filepath.Join(tmpDir, "data"))
	assert.True(t, os.IsNotExist(statErr), "expected no data directory on disk in stateless mode")

	// All migrations should be applied cleanly.
	statuses, err := ListMigrationStatus(db)
	require.NoError(t, err)
	require.NotEmpty(t, statuses)
	for _, st := range statuses {
		assert.True(t, st.Applied, "migration %s should be applied", st.ID)
	}

	// Basic CRUD works against the in-memory database.
	require.NoError(t, db.Create(&Whitelist{Domain: "example.com"}).Error)
	var count int64
	require.NoError(t, db.Model(&Whitelist{}).Where("domain = ?", "example.com").Count(&count).Error)
	assert.Equal(t, int64(1), count)
}
