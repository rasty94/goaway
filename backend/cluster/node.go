package cluster

import (
	"time"
)

type NodeRole string

const (
	RolePrimary NodeRole = "primary"
	RoleReplica NodeRole = "replica"
	RoleStandby NodeRole = "standby"
	RoleUnknown NodeRole = "unknown"
)

type NodeState string

const (
	StateOnline  NodeState = "online"
	StateOffline NodeState = "offline"
	StateSyncing NodeState = "syncing"
	StateUnknown NodeState = "unknown"
)

// ClusterNode represents a node in the cluster from this instance's perspective
type ClusterNode struct {
	ID          string    `json:"id"`
	Address     string    `json:"address"` // API endpoint (e.g. http://ip:8080)
	IP          string    `json:"ip"`      // Node IP for DNS/DHCP (e.g. 192.168.1.10)
	Role        NodeRole  `json:"role"`
	Status      NodeState `json:"status"`
	Priority    int       `json:"priority"`
	LastSeen    time.Time `json:"lastSeen"`
	LatencyMs   int64     `json:"latencyMs"`
	Version     string    `json:"version"`
	Unreachable bool      `json:"unreachable"`
	// PendingEvents is the replication backlog still queued for this node; NeedsResync means
	// events were dropped unsent and the node must be re-seeded from a backup.
	PendingEvents int64 `json:"pendingEvents"`
	NeedsResync   bool  `json:"needsResync"`
}

// HeartbeatRequest is the payload sent between nodes
type HeartbeatRequest struct {
	SourceID   string    `json:"sourceId"`
	SourceAddr string    `json:"sourceAddr"`
	Role       NodeRole  `json:"role"`
	Priority   int       `json:"priority"`
	Timestamp  time.Time `json:"timestamp"`
	Version    string    `json:"version"`
}

// HeartbeatResponse is the response to a heartbeat
type HeartbeatResponse struct {
	ID        string    `json:"id"`
	Role      NodeRole  `json:"role"`
	Priority  int       `json:"priority"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
}
