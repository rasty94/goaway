package key

import (
	"goaway/backend/database"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTestKeyService(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&database.APIKey{}))
	return NewService(NewRepository(db))
}

func TestAPIKeyCreateVerifyRedactDelete(t *testing.T) {
	svc := newTestKeyService(t)

	readKey, err := svc.CreateKey("reader", []string{"read", "metrics"})
	require.NoError(t, err)
	assert.Len(t, readKey, 64)

	adminKey, err := svc.CreateKey("super", []string{"admin"})
	require.NoError(t, err)

	// Duplicate name should fail
	_, err = svc.CreateKey("reader", []string{"read"})
	require.Error(t, err)

	assert.True(t, svc.VerifyKeyScope(readKey, "read"))
	assert.True(t, svc.VerifyKeyScope(readKey, "metrics"))
	assert.False(t, svc.VerifyKeyScope(readKey, "write"))
	assert.False(t, svc.VerifyKeyScope("nonexistent", "read"))

	// Admin scope grants any required scope
	assert.True(t, svc.VerifyKeyScope(adminKey, "write"))
	assert.True(t, svc.VerifyKeyScope(adminKey, "backup"))

	all, err := svc.GetAllKeys()
	require.NoError(t, err)
	require.Len(t, all, 2)
	for _, k := range all {
		assert.Equal(t, "redacted", k.Key)
	}

	require.NoError(t, svc.DeleteKey("reader"))
	assert.False(t, svc.VerifyKeyScope(readKey, "read"))
	assert.True(t, svc.VerifyKeyScope(adminKey, "read"))
}
