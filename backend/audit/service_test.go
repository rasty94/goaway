package audit

import (
	"goaway/backend/database"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestAuditCreateAndRead(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&database.Audit{}))

	svc := NewService(NewRepository(db))

	now := time.Now().Truncate(time.Second)
	svc.CreateAudit(&Entry{Topic: TopicAPI, Message: "first event", CreatedAt: now})
	svc.CreateAudit(&Entry{Topic: TopicDNS, Message: "second event", CreatedAt: now.Add(time.Second)})

	entries, err := svc.ReadAudits()
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "second event", entries[0].Message)
	assert.Equal(t, TopicDNS, entries[0].Topic)
	assert.Equal(t, "first event", entries[1].Message)
}
