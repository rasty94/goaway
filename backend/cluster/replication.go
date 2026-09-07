package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"goaway/backend/database"

	"gorm.io/gorm"
)

type EventType string

const (
	EventBlacklistAdd       EventType = "blacklist.add"
	EventBlacklistAddBatch  EventType = "blacklist.add.batch"
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

// Broadcast enqueues the event for every configured peer. Only a Primary with a secret emits events.
func (rm *ReplicationManager) Broadcast(event ReplicatedEvent) {
	if rm.service.selfRole != RolePrimary || rm.db == nil || rm.service.config.HighAvailability.Secret == "" {
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

// flush drains every peer's queue concurrently; each peer is still served strictly in order.
func (rm *ReplicationManager) flush() {
	var wg sync.WaitGroup
	for _, addr := range rm.service.peerAddresses() {
		wg.Add(1)
		go func(a string) {
			defer wg.Done()
			rm.flushPeer(a)
		}(addr)
	}
	wg.Wait()
	// ponytail: a peer removed from config or unreachable for a day loses its backlog; no automatic resync exists yet.
	rm.db.Where("created_at < ?", time.Now().Add(-outboxTTL)).Delete(&database.ReplicationOutbox{})
}

// flushPeer pushes rows in id order. A transport failure stops and retries later; a rejection
// the peer will never accept (400/500 on apply) is logged and dropped so it cannot block the queue.
func (rm *ReplicationManager) flushPeer(addr string) {
	for {
		var rows []database.ReplicationOutbox
		if err := rm.db.Where("peer = ?", addr).Order("id").Limit(outboxBatch).Find(&rows).Error; err != nil {
			log.Error("[HA/Replication] Cannot read outbox for %s: %v", addr, err)
			return
		}
		for _, row := range rows {
			permanent, err := rm.pushToPeer(addr, []byte(row.Event))
			if err != nil && !permanent {
				log.Debug("[HA/Replication] Peer %s unreachable, will retry: %v", addr, err)
				return
			}
			if err != nil {
				log.Warning("[HA/Replication] Peer %s rejected event, dropping it: %v", addr, err)
			}
			rm.db.Delete(&row)
		}
		if len(rows) < outboxBatch {
			return
		}
	}
}

// pushToPeer returns permanent=true when the peer answered but refused the event.
func (rm *ReplicationManager) pushToPeer(address string, payload []byte) (permanent bool, err error) {
	req, err := http.NewRequest(http.MethodPost, address+"/api/native/cluster/replicate", bytes.NewReader(payload))
	if err != nil {
		return true, err
	}
	req.Header.Set("Content-Type", "application/json")
	SignRequest(req, rm.service.config.HighAvailability.Secret, payload)

	resp, err := rm.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusOK:
		return false, nil
	case resp.StatusCode == http.StatusConflict, resp.StatusCode >= http.StatusBadGateway:
		// peer is primary right now (split-brain) or a proxy in front of it is down: try again later
		return false, fmt.Errorf("status %d", resp.StatusCode)
	default:
		return true, fmt.Errorf("status %d", resp.StatusCode)
	}
}
