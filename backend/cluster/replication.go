package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type EventType string

const (
	EventBlacklistAdd    EventType = "blacklist.add"
	EventBlacklistRemove EventType = "blacklist.remove"
	EventWhitelistAdd    EventType = "whitelist.add"
	EventWhitelistRemove EventType = "whitelist.remove"
	EventGroupCreate     EventType = "group.create"
	EventGroupUpdate     EventType = "group.update"
	EventGroupDelete     EventType = "group.delete"
	EventClientUpdate    EventType = "client.update"
	EventDHCPLeaseAdd    EventType = "dhcp.lease.add"
	EventDHCPLeaseRemove EventType = "dhcp.lease.remove"
	EventDHCPStaticAdd   EventType = "dhcp.static.add"
	EventDHCPStaticRemove EventType = "dhcp.static.remove"
)

type ReplicatedEvent struct {
	Type      EventType   `json:"type"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
}

// ReplicationManager handles pushing events to peers
type ReplicationManager struct {
	service *Service
	queue   chan ReplicatedEvent
	wg      sync.WaitGroup
}

func NewReplicationManager(service *Service) *ReplicationManager {
	return &ReplicationManager{
		service: service,
		queue:   make(chan ReplicatedEvent, 100),
	}
}

func (rm *ReplicationManager) Start() {
	go rm.processQueue()
}

func (rm *ReplicationManager) Broadcast(event ReplicatedEvent) {
	if rm.service.selfRole != RolePrimary {
		return // Only Primary broadcasts
	}
	
	event.Timestamp = time.Now()
	select {
	case rm.queue <- event:
	default:
		log.Warning("[HA/Replication] Queue full, dropping event: %s", event.Type)
	}
}

func (rm *ReplicationManager) processQueue() {
	for event := range rm.queue {
		rm.service.peersMu.RLock()
		peers := make([]string, 0, len(rm.service.peers))
		for addr, node := range rm.service.peers {
			if node.Status == StateOnline {
				peers = append(peers, addr)
			}
		}
		rm.service.peersMu.RUnlock()

		for _, addr := range peers {
			rm.wg.Add(1)
			go func(address string, e ReplicatedEvent) {
				defer rm.wg.Done()
				rm.pushToPeer(address, e)
			}(addr, event)
		}
		rm.wg.Wait()
	}
}

func (rm *ReplicationManager) pushToPeer(address string, event ReplicatedEvent) {
	payload, _ := json.Marshal(event)
	targetURL := fmt.Sprintf("%s/api/native/cluster/replicate", address)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Debug("[HA/Replication] Failed to push to %s: %v", address, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Debug("[HA/Replication] Peer %s returned status %d", address, resp.StatusCode)
	}
}
