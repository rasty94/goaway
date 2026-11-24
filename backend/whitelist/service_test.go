package whitelist

import (
	"goaway/backend/cluster"
	"goaway/backend/database"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type recordingReplicator struct {
	events []cluster.ReplicatedEvent
}

func (r *recordingReplicator) Broadcast(ev cluster.ReplicatedEvent) {
	r.events = append(r.events, ev)
}

func TestWhitelistAddMatchRemoveAndReplication(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&database.Whitelist{}))

	svc := NewService(NewRepository(db))
	rep := &recordingReplicator{}
	svc.SetReplicator(rep)

	require.NoError(t, svc.AddDomain("safe.example.com"))
	require.NoError(t, svc.AddDomain("*.trusted.org"))

	assert.True(t, svc.IsWhitelisted("safe.example.com"))
	assert.True(t, svc.IsWhitelisted("api.trusted.org"))
	assert.False(t, svc.IsWhitelisted("ads.other.com"))

	domains, err := svc.GetDomains()
	require.NoError(t, err)
	assert.Len(t, domains, 2)

	require.NoError(t, svc.RemoveDomain("safe.example.com"))
	assert.False(t, svc.IsWhitelisted("safe.example.com"))
	assert.True(t, svc.IsWhitelisted("api.trusted.org"))

	require.Len(t, rep.events, 3)
	assert.Equal(t, cluster.EventWhitelistAdd, rep.events[0].Type)
	assert.Equal(t, cluster.EventWhitelistAdd, rep.events[1].Type)
	assert.Equal(t, cluster.EventWhitelistRemove, rep.events[2].Type)
}
