package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionTrackerDiscardsStaleEventsAfterFailover(t *testing.T) {
	var tracker VersionTracker

	evNew := ReplicatedEvent{
		Type:    EventBlacklistAdd,
		Payload: map[string]any{"domain": "ads.example.com"},
		Version: 10,
	}
	evStale := ReplicatedEvent{
		Type:    EventBlacklistRemove,
		Payload: map[string]any{"domain": "ads.example.com"},
		Version: 7,
	}
	evDuplicate := ReplicatedEvent{
		Type:    EventBlacklistAdd,
		Payload: map[string]any{"domain": "ads.example.com"},
		Version: 10,
	}
	evNewer := ReplicatedEvent{
		Type:    EventBlacklistRemove,
		Payload: map[string]any{"domain": "ads.example.com"},
		Version: 11,
	}

	key := EventEntityKey(evNew)
	assert.Equal(t, "blacklist:ads.example.com", key)

	assert.True(t, tracker.ShouldApply(key, evNew.Version))
	assert.False(t, tracker.ShouldApply(EventEntityKey(evStale), evStale.Version))
	assert.False(t, tracker.ShouldApply(EventEntityKey(evDuplicate), evDuplicate.Version))
	assert.True(t, tracker.ShouldApply(EventEntityKey(evNewer), evNewer.Version))

	// After observing version 11, NextVersion on a promoted node must be > 11.
	assert.Equal(t, int64(12), tracker.NextVersion())

	// Unversioned legacy events (version 0) remain accepted for backwards compatibility.
	assert.True(t, tracker.ShouldApply(key, 0))
}
