package cluster

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"goaway/backend/database"
	"goaway/backend/settings"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestOutboxDeliversInOrderOnceThePeerIsBack(t *testing.T) {
	var mu sync.Mutex
	up := false
	var got []EventType
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		body, _ := io.ReadAll(r.Body)
		if !up {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if err := VerifyRequest(r.Header, "s3cret", r.URL.Path, body, time.Now()); err != nil {
			t.Errorf("peer received unsigned event: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var ev ReplicatedEvent
		_ = json.Unmarshal(body, &ev)
		got = append(got, ev.Type)
	}))
	defer peer.Close()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&database.ReplicationOutbox{}); err != nil {
		t.Fatal(err)
	}

	cfg := &settings.Config{}
	cfg.HighAvailability.Enabled = true
	cfg.HighAvailability.Mode = "primary"
	cfg.HighAvailability.Secret = "s3cret"
	cfg.HighAvailability.Peers = []string{peer.URL}

	s := NewService(cfg, "node-1", db)
	s.initializePeers()

	s.Broadcast(ReplicatedEvent{Type: EventBlacklistAdd, Payload: map[string]string{"domain": "a.test"}})
	s.Broadcast(ReplicatedEvent{Type: EventBlacklistRemove, Payload: map[string]string{"domain": "a.test"}})

	s.Replicator.flush()
	var pending int64
	db.Model(&database.ReplicationOutbox{}).Count(&pending)
	if pending != 2 {
		t.Fatalf("expected 2 queued events while peer is down, got %d", pending)
	}

	mu.Lock()
	up = true
	mu.Unlock()
	s.Replicator.flush()
	db.Model(&database.ReplicationOutbox{}).Count(&pending)
	if pending != 0 {
		t.Fatalf("expected empty outbox after delivery, got %d", pending)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 2 || got[0] != EventBlacklistAdd || got[1] != EventBlacklistRemove {
		t.Fatalf("events delivered out of order or missing: %v", got)
	}
}

func TestReplicaDoesNotQueue(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	_ = db.AutoMigrate(&database.ReplicationOutbox{})
	cfg := &settings.Config{}
	cfg.HighAvailability.Mode = "replica"
	cfg.HighAvailability.Peers = []string{"http://127.0.0.1:1"}
	s := NewService(cfg, "node-2", db)
	s.initializePeers()
	s.Broadcast(ReplicatedEvent{Type: EventBlacklistAdd})
	var n int64
	db.Model(&database.ReplicationOutbox{}).Count(&n)
	if n != 0 {
		t.Fatalf("replica queued %d events", n)
	}
}

func TestOutboxDropsEventsThePeerRefuses(t *testing.T) {
	var mu sync.Mutex
	var got []EventType
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		body, _ := io.ReadAll(r.Body)
		var ev ReplicatedEvent
		_ = json.Unmarshal(body, &ev)
		got = append(got, ev.Type)
		if ev.Type == EventGroupCreate {
			w.WriteHeader(http.StatusInternalServerError) // apply error: never going to succeed
		}
	}))
	defer peer.Close()

	db, _ := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	_ = db.AutoMigrate(&database.ReplicationOutbox{})
	cfg := &settings.Config{}
	cfg.HighAvailability.Mode = "primary"
	cfg.HighAvailability.Secret = "s3cret"
	cfg.HighAvailability.Peers = []string{peer.URL}
	s := NewService(cfg, "node-1", db)
	s.initializePeers()

	s.Broadcast(ReplicatedEvent{Type: EventGroupCreate})
	s.Broadcast(ReplicatedEvent{Type: EventBlacklistAdd})
	s.Replicator.flush()

	var pending int64
	db.Model(&database.ReplicationOutbox{}).Count(&pending)
	mu.Lock()
	defer mu.Unlock()
	if pending != 0 || len(got) != 2 || got[1] != EventBlacklistAdd {
		t.Fatalf("refused event should be dropped and the next one delivered: pending=%d got=%v", pending, got)
	}
}

func TestBroadcastQueuesBlacklistAddBatch(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	_ = db.AutoMigrate(&database.ReplicationOutbox{})
	cfg := &settings.Config{}
	cfg.HighAvailability.Mode = "primary"
	cfg.HighAvailability.Secret = "s3cret"
	cfg.HighAvailability.Peers = []string{"http://127.0.0.1:1"}
	s := NewService(cfg, "node-1", db)
	s.initializePeers()

	s.Broadcast(ReplicatedEvent{Type: EventBlacklistAddBatch, Payload: map[string][]string{"domains": {"a.test", "b.test"}}})

	var n int64
	db.Model(&database.ReplicationOutbox{}).Count(&n)
	if n != 1 {
		t.Fatalf("expected 1 queued batch event, got %d", n)
	}
}

func TestBroadcastIsOffWithoutSecret(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	_ = db.AutoMigrate(&database.ReplicationOutbox{})
	cfg := &settings.Config{}
	cfg.HighAvailability.Mode = "primary"
	cfg.HighAvailability.Peers = []string{"http://127.0.0.1:1"}
	s := NewService(cfg, "node-1", db)
	s.initializePeers()
	s.Broadcast(ReplicatedEvent{Type: EventBlacklistAdd})
	var n int64
	db.Model(&database.ReplicationOutbox{}).Count(&n)
	if n != 0 {
		t.Fatalf("queued %d events without a secret", n)
	}
}

func TestExpiredBacklogFlagsThePeerForResync(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	_ = db.AutoMigrate(&database.ReplicationOutbox{})
	cfg := &settings.Config{}
	cfg.HighAvailability.Mode = "primary"
	cfg.HighAvailability.Secret = "s3cret"
	cfg.HighAvailability.Peers = []string{"http://127.0.0.1:1"}
	s := NewService(cfg, "node-1", db)
	s.initializePeers()

	s.Broadcast(ReplicatedEvent{Type: EventBlacklistAdd})
	db.Model(&database.ReplicationOutbox{}).Where("1 = 1").
		Update("created_at", time.Now().Add(-2*outboxTTL))

	nodes := s.GetNodes()
	if len(nodes) != 1 || nodes[0].PendingEvents != 1 || nodes[0].NeedsResync {
		t.Fatalf("before expiry: %+v", nodes[0])
	}

	s.Replicator.expireBacklog()

	var left int64
	db.Model(&database.ReplicationOutbox{}).Count(&left)
	nodes = s.GetNodes()
	if left != 0 || !nodes[0].NeedsResync || nodes[0].PendingEvents != 0 {
		t.Fatalf("after expiry: left=%d node=%+v", left, nodes[0])
	}

	s.ClearResync("http://127.0.0.1:1")
	if s.GetNodes()[0].NeedsResync {
		t.Fatal("ClearResync did not clear the flag")
	}
}

func TestRoleChangesAreSafeUnderConcurrentReads(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	_ = db.AutoMigrate(&database.ReplicationOutbox{})
	cfg := &settings.Config{}
	cfg.HighAvailability.Mode = "replica"
	cfg.HighAvailability.Secret = "s3cret"
	s := NewService(cfg, "node-1", db)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			s.evaluateLeader()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_ = s.IsPrimary()
			_ = s.Role()
		}
	}()
	wg.Wait()

	// No peers, so this node wins its own election.
	if !s.IsPrimary() {
		t.Fatalf("expected the only node to elect itself, got %s", s.Role())
	}
}
