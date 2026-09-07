package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"goaway/backend/logging"
	"goaway/backend/settings"
	"net/http"
	"sort"
	"sync"
	"time"

	"gorm.io/gorm"
)

var log = logging.GetLogger()

// Replicator interface for services to trigger data replication
type Replicator interface {
	Broadcast(event ReplicatedEvent)
}

// Service handles membership and leader election for HA Active
type Service struct {
	config  *settings.Config
	peers   map[string]*ClusterNode
	peersMu sync.RWMutex

	Replicator *ReplicationManager

	selfID   string
	selfRole NodeRole

	needsResync map[string]bool

	stopCh   chan struct{}
	stopOnce sync.Once
}

func NewService(config *settings.Config, selfID string, db *gorm.DB) *Service {
	s := &Service{
		config:      config,
		peers:       make(map[string]*ClusterNode),
		needsResync: make(map[string]bool),
		selfID:      selfID,
		selfRole:    NodeRole(config.HighAvailability.Mode),
		stopCh:      make(chan struct{}),
	}
	s.Replicator = NewReplicationManager(s, db)
	return s
}

// Start begins node monitoring and heartbeats
func (s *Service) Start() {
	if !s.config.HighAvailability.Enabled {
		log.Info("[HA/Cluster] Active clustering disabled")
		return
	}

	log.Info("[HA/Cluster] Starting clustering service (Mode: %s, Priority: %d)",
		s.selfRole, s.config.HighAvailability.Priority)

	if s.config.HighAvailability.Secret == "" {
		log.Error("[HA/Cluster] highAvailability.secret is empty: clustering stays off until the same secret is set on every node")
		return
	}

	s.Replicator.Start()

	// Initialize peers from config
	s.initializePeers()

	go s.heartbeatLoop()
}

func (s *Service) initializePeers() {
	s.peersMu.Lock()
	defer s.peersMu.Unlock()

	for _, peerAddr := range s.config.HighAvailability.Peers {
		s.peers[peerAddr] = &ClusterNode{
			Address:  peerAddr,
			Status:   StateUnknown,
			Role:     RoleUnknown,
			Priority: 0,
		}
	}
}

func (s *Service) heartbeatLoop() {
	ticker := time.NewTicker(10 * time.Second) // configurable in future
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.pingPeers()
			s.evaluateLeader()
		}
	}
}

func (s *Service) pingPeers() {
	s.peersMu.RLock()
	peers := make([]string, 0, len(s.peers))
	for addr := range s.peers {
		peers = append(peers, addr)
	}
	s.peersMu.RUnlock()

	var wg sync.WaitGroup
	for _, addr := range peers {
		wg.Add(1)
		go func(address string) {
			defer wg.Done()
			s.pingNode(address)
		}(addr)
	}
	wg.Wait()
}

func (s *Service) pingNode(address string) {
	startTime := time.Now()

	req := HeartbeatRequest{
		SourceID:  s.selfID,
		Role:      s.selfRole,
		Priority:  s.config.HighAvailability.Priority,
		Timestamp: time.Now(),
		Version:   "1.0.0", // update for actual version
	}

	payload, _ := json.Marshal(req)

	// Create request to peer's heartbeat URL
	targetURL := fmt.Sprintf("%s/api/native/cluster/heartbeat", address)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	httpReq, _ := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewBuffer(payload))
	httpReq.Header.Set("Content-Type", "application/json")
	SignRequest(httpReq, s.config.HighAvailability.Secret, payload)

	client := &http.Client{}
	resp, err := client.Do(httpReq)

	latency := time.Since(startTime).Milliseconds()

	s.peersMu.Lock()
	defer s.peersMu.Unlock()

	node, exists := s.peers[address]
	if !exists {
		return
	}

	if err != nil {
		node.Status = StateOffline
		node.Unreachable = true
		log.Debug("[HA/Cluster] Peer %s unreachable: %v", address, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var hbResp HeartbeatResponse
		if err := json.NewDecoder(resp.Body).Decode(&hbResp); err == nil {
			node.Status = StateOnline
			node.Role = hbResp.Role
			node.ID = hbResp.ID
			node.Priority = hbResp.Priority
			node.LastSeen = time.Now()
			node.LatencyMs = latency
			node.Unreachable = false
		}
	} else {
		node.Status = StateOffline
		node.Unreachable = true
	}
}

// evaluateLeader checks if current node should be the Primary
func (s *Service) evaluateLeader() {
	s.peersMu.RLock()
	defer s.peersMu.RUnlock()

	// Simple logic: node with highest priority is Primary.
	// In case of tie, node with alphabetical lower ID is Primary.

	bestPriority := s.config.HighAvailability.Priority
	bestID := s.selfID
	bestAddress := "local"

	hasLeader := false
	for _, node := range s.peers {
		if node.Status == StateOnline && node.Role == RolePrimary {
			hasLeader = true
		}

		if node.Status == StateOnline && node.Priority > bestPriority {
			bestPriority = node.Priority
			bestID = node.ID
			bestAddress = node.Address
		} else if node.Status == StateOnline && node.Priority == bestPriority && node.ID < bestID {
			bestID = node.ID
			bestAddress = node.Address
		}
	}

	if bestID == s.selfID {
		if s.selfRole != RolePrimary {
			log.Warning("[HA/Cluster] Elected as NEW PRIMARY (ID: %s)", s.selfID)
			s.selfRole = RolePrimary
			// Update config or notify other services
		}
	} else {
		if s.selfRole == RolePrimary {
			log.Warning("[HA/Cluster] Relinquishing Primary role to %s (ID: %s)", bestAddress, bestID)
			s.selfRole = RoleReplica
		}
	}

	if !hasLeader && bestID != s.selfID {
		log.Warning("[HA/Cluster] No leader detected in cluster, waiting for consensus...")
	}
}

// peerAddresses returns every configured peer, reachable or not, so queued events survive an outage.
func (s *Service) peerAddresses() []string {
	s.peersMu.RLock()
	defer s.peersMu.RUnlock()
	addrs := make([]string, 0, len(s.peers))
	for addr := range s.peers {
		addrs = append(addrs, addr)
	}
	sort.Strings(addrs)
	return addrs
}

func (s *Service) SelfID() string { return s.selfID }

func (s *Service) IsPrimary() bool {
	s.peersMu.RLock()
	defer s.peersMu.RUnlock()
	return s.selfRole == RolePrimary
}

// markNeedsResync records that a peer lost events and can no longer be brought up to date incrementally.
func (s *Service) markNeedsResync(addr string) {
	s.peersMu.Lock()
	defer s.peersMu.Unlock()
	s.needsResync[addr] = true
}

// ClearResync is called once an operator has re-seeded the node from a backup.
func (s *Service) ClearResync(addr string) {
	s.peersMu.Lock()
	defer s.peersMu.Unlock()
	delete(s.needsResync, addr)
}

func (s *Service) GetNodes() []*ClusterNode {
	pending := make(map[string]int64)
	for _, addr := range s.peerAddresses() {
		pending[addr] = s.Replicator.pending(addr)
	}

	s.peersMu.RLock()
	defer s.peersMu.RUnlock()

	nodes := make([]*ClusterNode, 0, len(s.peers))
	for addr, node := range s.peers {
		copied := *node
		copied.PendingEvents = pending[addr]
		copied.NeedsResync = s.needsResync[addr]
		nodes = append(nodes, &copied)
	}
	return nodes
}

func (s *Service) Broadcast(event ReplicatedEvent) {
	s.Replicator.Broadcast(event)
}

func (s *Service) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}
