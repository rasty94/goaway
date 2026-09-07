package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"goaway/backend/database"

	"gorm.io/gorm"
)

type EventType string

const (
	EventBlacklistAdd       EventType = "blacklist.add"
	EventBlacklistRemove    EventType = "blacklist.remove"
	EventWhitelistAdd       EventType = "whitelist.add"
	EventWhitelistRemove    EventType = "whitelist.remove"
	EventGroupCreate        EventType = "group.create"
	EventGroupUpdate        EventType = "group.update"
	EventGroupDelete        EventType = "group.delete"
	EventGroupBlockedAdd    EventType = "group.blocked.add"
	EventGroupBlockedRemove EventType = "group.blocked.remove"
	EventGroupAllowedAdd    EventType = "group.allowed.add"
	EventGroupAllowedRemove EventType = "group.allowed.remove"
	EventClientUpdate       EventType = "client.update"
	EventResolutionAdd      EventType = "resolution.add"
	EventResolutionRemove   EventType = "resolution.remove"
	EventDHCPLeaseAdd       EventType = "dhcp.lease.add"
	EventDHCPLeaseRemove    EventType = "dhcp.lease.remove"
	EventDHCPLeasev6Add     EventType = "dhcp.lease6.add"
	EventDHCPStaticAdd      EventType = "dhcp.static.add"
	EventDHCPStaticRemove   EventType = "dhcp.static.remove"
)

type ReplicatedEvent struct {
	Type      EventType   `json:"type"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
}

const (
	outboxBatch    = 100
	outboxTTL      = 24 * time.Hour
	outboxInterval = 5 * time.Second
)

// ReplicationManager persists every event per peer and pushes them in order until the peer acks.
type ReplicationManager struct {
	service *Service
	db      *gorm.DB
	wake    chan struct{}
	client  *http.Client
}

func NewReplicationManager(service *Service, db *gorm.DB) *ReplicationManager {
	return &ReplicationManager{
		service: service,
		db:      db,
		wake:    make(chan struct{}, 1),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (rm *ReplicationManager) Start() {
	if rm.db == nil {
		log.Error("[HA/Replication] No database handle: events will not be replicated")
		return
	}
	go rm.loop()
}

// Broadcast enqueues the event for every configured peer. Only the Primary emits events.
func (rm *ReplicationManager) Broadcast(event ReplicatedEvent) {
	if rm.service.selfRole != RolePrimary || rm.db == nil {
		return
	}

	event.Timestamp = time.Now()
	payload, err := json.Marshal(event)
	if err != nil {
		log.Error("[HA/Replication] Cannot encode event %s: %v", event.Type, err)
		return
	}

	peers := rm.service.peerAddresses()
	if len(peers) == 0 {
		return
	}
	rows := make([]database.ReplicationOutbox, 0, len(peers))
	for _, addr := range peers {
		rows = append(rows, database.ReplicationOutbox{Peer: addr, Event: string(payload)})
	}
	if err := rm.db.Create(&rows).Error; err != nil {
		log.Error("[HA/Replication] Cannot queue event %s: %v", event.Type, err)
		return
	}

	select {
	case rm.wake <- struct{}{}:
	default:
	}
}

func (rm *ReplicationManager) loop() {
	ticker := time.NewTicker(outboxInterval)
	defer ticker.Stop()
	for {
		select {
		case <-rm.service.stopCh:
			return
		case <-rm.wake:
		case <-ticker.C:
		}
		rm.flush()
	}
}

// flush pushes pending events peer by peer, stopping at the first failure so ordering is preserved.
func (rm *ReplicationManager) flush() {
	for _, addr := range rm.service.peerAddresses() {
		rm.flushPeer(addr)
	}
	// ponytail: a peer removed from config or dead for a day loses its backlog; it re-syncs via the Phase 1 backup pull.
	rm.db.Where("created_at < ?", time.Now().Add(-outboxTTL)).Delete(&database.ReplicationOutbox{})
}

func (rm *ReplicationManager) flushPeer(addr string) {
	var rows []database.ReplicationOutbox
	if err := rm.db.Where("peer = ?", addr).Order("id").Limit(outboxBatch).Find(&rows).Error; err != nil {
		log.Error("[HA/Replication] Cannot read outbox for %s: %v", addr, err)
		return
	}
	for _, row := range rows {
		if err := rm.pushToPeer(addr, []byte(row.Event)); err != nil {
			log.Debug("[HA/Replication] Peer %s not acking (attempt %d): %v", addr, row.Attempts+1, err)
			rm.db.Model(&row).Update("attempts", row.Attempts+1)
			return
		}
		rm.db.Delete(&row)
	}
}

func (rm *ReplicationManager) pushToPeer(address string, payload []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, address+"/api/native/cluster/replicate", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	SignRequest(req, rm.service.config.HighAvailability.Secret, payload)

	resp, err := rm.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}
