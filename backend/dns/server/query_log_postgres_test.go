package server

import (
	model "goaway/backend/dns/server/models"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestInsertQueryLogBatchTransactional(t *testing.T) {
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)

	sqlDB, err := gormDB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	entries := []model.RequestLogEntry{sampleLogEntry(), sampleLogEntry()}
	require.NoError(t, insertQueryLogBatch(sqlDB, entries))

	var count int
	require.NoError(t, sqlDB.QueryRow("SELECT COUNT(*) FROM goaway_query_logs").Scan(&count))
	assert.Equal(t, 2, count)

	var domain, proto string
	require.NoError(t, sqlDB.QueryRow("SELECT domain, protocol FROM goaway_query_logs LIMIT 1").Scan(&domain, &proto))
	assert.Equal(t, "example.com.", domain)
	assert.Equal(t, "DoQ", proto)
}
