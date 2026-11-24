package prefetch

import (
	"goaway/backend/database"
	"goaway/backend/dns/server"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPrefetchAddLoadRemoveAndExpiry(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&database.Prefetch{}))

	dnsSrv := &server.DNSServer{}
	svc := NewService(NewRepository(db), dnsSrv)

	require.NoError(t, svc.AddPrefetchedDomain("example.com.", 60, 1))
	require.Contains(t, svc.Domains, "example.com.")
	assert.Equal(t, 60, svc.Domains["example.com."].Refresh)
	assert.Equal(t, "example.com.:A", svc.buildCacheKey("example.com.", "A"))

	// Reload in a fresh service instance to verify persistence
	svc2 := NewService(NewRepository(db), dnsSrv)
	assert.Contains(t, svc2.Domains, "example.com.")

	now := time.Now()
	assert.True(t, svc.isExpired(server.CachedRecord{ExpiresAt: now.Add(-time.Second)}, now))
	assert.True(t, svc.isExpired(server.CachedRecord{ExpiresAt: now}, now))
	assert.False(t, svc.isExpired(server.CachedRecord{ExpiresAt: now.Add(time.Minute)}, now))

	require.NoError(t, svc.RemovePrefetchedDomain("example.com."))
	assert.NotContains(t, svc.Domains, "example.com.")
}
