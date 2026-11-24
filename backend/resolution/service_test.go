package resolution

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

func (r *recordingReplicator) Broadcast(event cluster.ReplicatedEvent) {
	r.events = append(r.events, event)
}

func newTestResolutionService(t *testing.T) (*Service, *recordingReplicator) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&database.Resolution{}))

	svc := NewService(NewRepository(db))
	rep := &recordingReplicator{}
	svc.SetReplicator(rep)
	return svc, rep
}

func TestResolutionCRUDAndReplication(t *testing.T) {
	svc, rep := newTestResolutionService(t)

	require.NoError(t, svc.CreateResolution("192.168.1.50", "nas.lan", ""))
	require.NoError(t, svc.CreateResolution("fd00::50", "nas6.lan", "AAAA"))

	res, err := svc.GetResolution("nas.lan")
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.50", res.Value)
	assert.Equal(t, "A", res.Type)

	all, err := svc.GetResolutions()
	require.NoError(t, err)
	assert.Len(t, all, 2)

	deleted, err := svc.DeleteResolution("192.168.1.50", "nas.lan")
	require.NoError(t, err)
	assert.Equal(t, 1, deleted)

	require.Len(t, rep.events, 3)
	assert.Equal(t, cluster.EventResolutionAdd, rep.events[0].Type)
	assert.Equal(t, cluster.EventResolutionAdd, rep.events[1].Type)
	assert.Equal(t, cluster.EventResolutionRemove, rep.events[2].Type)
}
