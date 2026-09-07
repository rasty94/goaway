package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"goaway/backend/blacklist"
	"goaway/backend/cluster"
	"goaway/backend/database"
	"goaway/backend/group"
	"goaway/backend/resolution"
	"goaway/backend/settings"
	"goaway/backend/whitelist"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const clusterTestSecret = "integration-s3cret"

var clusterNodeSeq atomic.Int64

// haNode is one goaway instance: its own database, the real services, an *API and the two
// node-to-node routes behind the real clusterAuth middleware.
type haNode struct {
	DB      *gorm.DB
	API     *API
	Cluster *cluster.Service
	Server  *httptest.Server
	URL     string
}

type nodeSpec struct {
	id       string
	role     string // "primary" or "replica"
	priority int
	peers    []string
	listener net.Listener // optional: bind the node's server to an address the test already knows
}

// reserveAddr returns a listener on a free loopback port plus the URL a peer would use to reach it.
func reserveAddr(t *testing.T) (net.Listener, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve address: %v", err)
	}
	return ln, "http://" + ln.Addr().String()
}

func newHANode(t *testing.T, spec nodeSpec) *haNode {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:cluster-it-%d?mode=memory&cache=shared", clusterNodeSeq.Add(1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		TranslateError: true,
		Logger:         gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// The "Custom" source marks the blacklist as already initialized, so NewService skips the
	// default blocklist download. Tests must not touch the network.
	if err := db.Create(&database.Source{Name: "Custom", Active: true, LastUpdated: time.Now()}).Error; err != nil {
		t.Fatalf("seed custom source: %v", err)
	}

	cfg := &settings.Config{}
	cfg.HighAvailability.Enabled = true
	cfg.HighAvailability.Mode = spec.role
	cfg.HighAvailability.Priority = spec.priority
	cfg.HighAvailability.Peers = spec.peers
	cfg.HighAvailability.Secret = clusterTestSecret

	// Same construction order as services/registry.go, minus the DNS server and DHCP service:
	// no cluster event exercised here needs them.
	api := &API{
		Config:            cfg,
		DBConn:            db,
		BlacklistService:  blacklist.NewService(blacklist.NewRepository(db)),
		WhitelistService:  whitelist.NewService(whitelist.NewRepository(db)),
		GroupService:      group.NewService(group.NewRepository(db)),
		ResolutionService: resolution.NewService(resolution.NewRepository(db)),
	}

	cs := cluster.NewService(cfg, spec.id, db)
	api.ClusterManager = cs
	api.BlacklistService.SetReplicator(cs)
	api.WhitelistService.SetReplicator(cs)
	api.GroupService.SetReplicator(cs)
	api.ResolutionService.SetReplicator(cs)

	// Start() is the production path that loads the peers from config; Stop() immediately kills the
	// 10s heartbeat and 5s outbox tickers so the tests drive Tick()/Flush() themselves and nothing
	// fires behind their back.
	cs.Start()
	cs.Stop()

	r := gin.New()
	native := r.Group("/api/native/cluster", api.clusterAuth())
	native.POST("/heartbeat", api.handleHeartbeat)
	native.POST("/replicate", api.handleReplication)

	srv := httptest.NewUnstartedServer(r)
	if spec.listener != nil {
		_ = srv.Listener.Close()
		srv.Listener = spec.listener
	}
	srv.Start()
	t.Cleanup(srv.Close)

	return &haNode{DB: db, API: api, Cluster: cs, Server: srv, URL: srv.URL}
}

func (n *haNode) outboxCount(t *testing.T) int64 {
	t.Helper()
	var count int64
	if err := n.DB.Model(&database.ReplicationOutbox{}).Count(&count).Error; err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	return count
}

func (n *haNode) outboxEvents(t *testing.T) []cluster.EventType {
	t.Helper()
	var rows []database.ReplicationOutbox
	if err := n.DB.Order("id").Find(&rows).Error; err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	types := make([]cluster.EventType, 0, len(rows))
	for _, row := range rows {
		var ev cluster.ReplicatedEvent
		if err := json.Unmarshal([]byte(row.Event), &ev); err != nil {
			t.Fatalf("decode queued event: %v", err)
		}
		types = append(types, ev.Type)
	}
	return types
}

func (n *haNode) blacklistCount(t *testing.T, domain string) int64 {
	t.Helper()
	var count int64
	if err := n.DB.Model(&database.Blacklist{}).Where("domain = ?", domain).Count(&count).Error; err != nil {
		t.Fatalf("count blacklist: %v", err)
	}
	return count
}

// postEvent sends a replication event to the node, signed with the given secret ("" means no
// signature headers at all).
func postEvent(t *testing.T, target *haNode, secret string, event cluster.ReplicatedEvent) *http.Response {
	t.Helper()
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, target.URL+"/api/native/cluster/replicate", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		cluster.SignRequest(req, secret, body)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post event: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func TestClusterReplicatesBlacklistEndToEnd(t *testing.T) {
	replicaLn, replicaURL := reserveAddr(t)
	replica := newHANode(t, nodeSpec{id: "replica-1", role: "replica", priority: 1, listener: replicaLn})
	primary := newHANode(t, nodeSpec{id: "primary-1", role: "primary", priority: 10, peers: []string{replicaURL}})

	if err := primary.API.BlacklistService.AddBlacklistedDomain(context.Background(), "ads.test"); err != nil {
		t.Fatalf("add domain on primary: %v", err)
	}
	if queued := primary.outboxCount(t); queued != 1 {
		t.Fatalf("expected the real service to queue 1 event, got %d", queued)
	}

	primary.Cluster.Replicator.Flush()

	if n := replica.blacklistCount(t, "ads.test"); n != 1 {
		t.Fatalf("replica database has %d rows for ads.test, want 1", n)
	}
	if !replica.API.BlacklistService.IsBlacklisted("ads.test") {
		t.Fatalf("replica blacklist service does not report ads.test as blocked")
	}
	if queued := primary.outboxCount(t); queued != 0 {
		t.Fatalf("primary outbox still holds %d rows after a successful flush", queued)
	}
}

func TestReplicaRejectsUnsignedAndWronglySignedEvents(t *testing.T) {
	replica := newHANode(t, nodeSpec{id: "replica-1", role: "replica", priority: 1})
	event := cluster.ReplicatedEvent{
		Type:    cluster.EventBlacklistAdd,
		Payload: map[string]string{"domain": "evil.test"},
	}

	for name, secret := range map[string]string{"unsigned": "", "wrong secret": "not-the-secret"} {
		resp := postEvent(t, replica, secret, event)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s event: expected 401, got %d", name, resp.StatusCode)
		}
		if n := replica.blacklistCount(t, "evil.test"); n != 0 {
			t.Fatalf("%s event changed replica state: %d rows for evil.test", name, n)
		}
		if replica.API.BlacklistService.IsBlacklisted("evil.test") {
			t.Fatalf("%s event reached the blacklist cache", name)
		}
	}
}

func TestPrimaryRefusesReplicatedEvents(t *testing.T) {
	primary := newHANode(t, nodeSpec{id: "primary-1", role: "primary", priority: 10})
	if !primary.Cluster.IsPrimary() {
		t.Fatalf("node built as primary does not report itself primary")
	}

	resp := postEvent(t, primary, clusterTestSecret, cluster.ReplicatedEvent{
		Type:    cluster.EventBlacklistAdd,
		Payload: map[string]string{"domain": "foreign.test"},
	})

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 from a primary, got %d", resp.StatusCode)
	}
	if n := primary.blacklistCount(t, "foreign.test"); n != 0 {
		t.Fatalf("primary applied a replicated event: %d rows for foreign.test", n)
	}
}

func TestElectionPrefersHigherPriority(t *testing.T) {
	lowLn, lowURL := reserveAddr(t)
	highLn, highURL := reserveAddr(t)

	// Both boot believing they are primary: the election has to resolve the split brain.
	low := newHANode(t, nodeSpec{id: "node-low", role: "primary", priority: 10, peers: []string{highURL}, listener: lowLn})
	high := newHANode(t, nodeSpec{id: "node-high", role: "primary", priority: 100, peers: []string{lowURL}, listener: highLn})

	low.Cluster.Tick()
	high.Cluster.Tick()

	if !high.Cluster.IsPrimary() {
		t.Fatalf("node with priority 100 did not stay primary")
	}
	if low.Cluster.IsPrimary() {
		t.Fatalf("node with priority 10 still considers itself primary; both nodes are primary (split brain)")
	}

	nodes := low.Cluster.GetNodes()
	if len(nodes) != 1 || nodes[0].Role != cluster.RolePrimary || nodes[0].ID != "node-high" {
		t.Fatalf("low-priority node did not learn the peer identity/role: %+v", nodes)
	}
}

func TestOutboxSurvivesAPeerOutage(t *testing.T) {
	ln, peerURL := reserveAddr(t)
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil { // peer is down: connections are refused
		t.Fatalf("close reserved listener: %v", err)
	}

	primary := newHANode(t, nodeSpec{id: "primary-1", role: "primary", priority: 10, peers: []string{peerURL}})
	if err := primary.API.BlacklistService.AddBlacklistedDomain(context.Background(), "first.test"); err != nil {
		t.Fatalf("add first domain: %v", err)
	}
	if err := primary.API.BlacklistService.RemoveDomain(context.Background(), "first.test"); err != nil {
		t.Fatalf("remove first domain: %v", err)
	}

	primary.Cluster.Replicator.Flush()

	queued := primary.outboxEvents(t)
	if len(queued) != 2 || queued[0] != cluster.EventBlacklistAdd || queued[1] != cluster.EventBlacklistRemove {
		t.Fatalf("events lost or reordered while the peer was down: %v", queued)
	}

	reopened, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("reopen peer address %s: %v", addr, err)
	}
	replica := newHANode(t, nodeSpec{id: "replica-1", role: "replica", priority: 1, listener: reopened})

	primary.Cluster.Replicator.Flush()

	if left := primary.outboxCount(t); left != 0 {
		t.Fatalf("outbox still holds %d rows after the peer came back", left)
	}
	// add then remove, applied in that order, leaves the replica without the domain
	if n := replica.blacklistCount(t, "first.test"); n != 0 {
		t.Fatalf("replica has %d rows for first.test: events applied out of order", n)
	}
	var applied int64
	if err := replica.DB.Model(&database.Blacklist{}).Count(&applied).Error; err != nil {
		t.Fatalf("count replica blacklist: %v", err)
	}
	if applied != 0 {
		t.Fatalf("replica blacklist should be empty after add+remove, got %d rows", applied)
	}
}
