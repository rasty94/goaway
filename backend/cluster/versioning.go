package cluster

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// VersionTracker implements a Lamport logical clock and per-entity high-water mark
// so replicas discard stale replicated events after a leader change or split-brain.
// Its zero value is ready to use.
type VersionTracker struct {
	clock atomic.Int64

	mu         sync.Mutex
	entityHigh map[string]int64
}

// NextVersion advances and returns the next logical clock version.
func (v *VersionTracker) NextVersion() int64 {
	return v.clock.Add(1)
}

// Observe advances the local clock to at least incoming (Lamport clock rule).
func (v *VersionTracker) Observe(incoming int64) {
	for {
		cur := v.clock.Load()
		if incoming <= cur {
			return
		}
		if v.clock.CompareAndSwap(cur, incoming) {
			return
		}
	}
}

// ShouldApply reports whether an incoming event version is newer than the last applied
// version for entityKey. Unversioned legacy events (version <= 0) are always accepted.
func (v *VersionTracker) ShouldApply(entityKey string, version int64) bool {
	if version <= 0 {
		return true
	}

	v.Observe(version)

	v.mu.Lock()
	defer v.mu.Unlock()

	if v.entityHigh == nil {
		v.entityHigh = make(map[string]int64)
	}

	if prev, ok := v.entityHigh[entityKey]; ok && version <= prev {
		return false
	}
	v.entityHigh[entityKey] = version
	return true
}

// EventEntityKey derives a stable conflict-resolution key for a replicated event.
func EventEntityKey(ev ReplicatedEvent) string {
	m, _ := ev.Payload.(map[string]any)
	switch ev.Type {
	case EventBlacklistAdd, EventBlacklistRemove:
		if d, ok := m["domain"].(string); ok && d != "" {
			return "blacklist:" + d
		}
	case EventWhitelistAdd, EventWhitelistRemove:
		if d, ok := m["domain"].(string); ok && d != "" {
			return "whitelist:" + d
		}
	case EventResolutionAdd, EventResolutionRemove:
		if d, ok := m["domain"].(string); ok && d != "" {
			return "resolution:" + d
		}
	case EventGroupCreate, EventGroupUpdate, EventGroupDelete:
		if name, ok := m["name"].(string); ok && name != "" {
			return "group:" + name
		}
		if id, ok := m["id"]; ok {
			return fmt.Sprintf("group:%v", id)
		}
	case EventClientUpdate:
		if id, ok := m["identifier"].(string); ok && id != "" {
			return "client:" + id
		}
	case EventDHCPStaticAdd, EventDHCPStaticRemove, EventDHCPLeaseAdd, EventDHCPLeaseRemove:
		if mac, ok := m["mac"].(string); ok && mac != "" {
			return "dhcp:" + mac
		}
	}
	return string(ev.Type)
}
