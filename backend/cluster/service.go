package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"goaway/backend/logging"
	"goaway/backend/settings"
	"io"
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

	selfID string

	// roleMu guards selfRole, which the election goroutine writes while request
	// goroutines read it through IsPrimary.
	roleMu   sync.RWMutex
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
			s.Tick()
		}
	}
}

// Tick runs one heartbeat + election round. Exported so tests can drive a round
// without waiting on the ticker.
func (s *Service) Tick() {
	s.pingPeers()
	s.evaluateLeader()
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
		Role:      s.Role(),
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

	client, err := NewPeerHTTPClient(s.config.HighAvailability.TLS, 5*time.Second)
	if err != nil {
		log.Warning("[HA/Cluster] Falling back to default heartbeat HTTP client: %v", err)
		client = &http.Client{Timeout: 5 * time.Second}
	}
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if readErr != nil {
			node.Status = StateOffline
			node.Unreachable = true
			return
		}
		if sigErr := VerifyRequest(resp.Header, s.config.HighAvailability.Secret, "/api/native/cluster/heartbeat", respBody, time.Now()); sigErr != nil {
			log.Warning("[HA/Cluster] Rejected unsigned or invalid heartbeat response from %s: %v", address, sigErr)
			node.Status = StateOffline
			node.Unreachable = true
			return
		}
		var hbResp HeartbeatResponse
		if err := json.Unmarshal(respBody, &hbResp); err == nil {
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

	s.peersMu.RUnlock()

	if bestID == s.selfID {
		if s.setRole(RolePrimary) {
			log.Warning("[HA/Cluster] Elected as NEW PRIMARY (ID: %s)", s.selfID)
		}
	} else if s.IsPrimary() {
		log.Warning("[HA/Cluster] Relinquishing Primary role to %s (ID: %s)", bestAddress, bestID)
		s.setRole(RoleReplica)
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

// Role reports the role this node currently holds.
func (s *Service) Role() NodeRole {
	s.roleMu.RLock()
	defer s.roleMu.RUnlock()
	return s.selfRole
}

func (s *Service) IsPrimary() bool {
	s.roleMu.RLock()
	defer s.roleMu.RUnlock()
	return s.selfRole == RolePrimary
}

// setRole records a role change and reports whether it actually changed.
func (s *Service) setRole(role NodeRole) bool {
	s.roleMu.Lock()
	defer s.roleMu.Unlock()
	if s.selfRole == role {
		return false
	}
	s.selfRole = role
	return true
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
