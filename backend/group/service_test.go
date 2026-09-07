package group

import (
	"testing"

	"goaway/backend/database"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&database.ClientGroup{}, &database.ClientGroupAssignment{},
		&database.GroupBlockedDomain{}, &database.GroupAllowedDomain{}); err != nil {
		t.Fatal(err)
	}
	return NewService(NewRepository(db))
}

func TestAddBlockedDomainsRefreshesCache(t *testing.T) {
	s := newTestService(t)

	g, err := s.CreateGroup("kids", "", false)
	if err != nil {
		t.Fatal(err)
	}
	inserted, err := s.AddBlockedDomains(g.ID, []string{"ads.example", "tracker.example", "junk.example"})
	if err != nil {
		t.Fatal(err)
	}
	if inserted != 3 {
		t.Fatalf("expected 3 domains inserted, got %d", inserted)
	}
	if err := s.ReplaceAssignments("10.0.0.5", "ip", []uint{g.ID}); err != nil {
		t.Fatal(err)
	}

	if !s.ShouldBlock("10.0.0.5", "", "ads.example", "ads.example", false, false) {
		t.Fatal("expected ads.example to be blocked after AddBlockedDomains, cache was not refreshed")
	}
}

func TestAddBlockedDomainsReportsPartialInsertOnDuplicate(t *testing.T) {
	s := newTestService(t)

	g, err := s.CreateGroup("kids", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddBlockedDomains(g.ID, []string{"ads.example"}); err != nil {
		t.Fatal(err)
	}

	inserted, err := s.AddBlockedDomains(g.ID, []string{"ads.example", "new.example"})
	if err == nil {
		t.Fatal("expected an error for the duplicate domain")
	}
	if inserted != 1 {
		t.Fatalf("expected 1 domain inserted, got %d", inserted)
	}

	if err := s.ReplaceAssignments("10.0.0.6", "ip", []uint{g.ID}); err != nil {
		t.Fatal(err)
	}
	if !s.ShouldBlock("10.0.0.6", "", "new.example", "new.example", false, false) {
		t.Fatal("expected new.example to be blocked despite the duplicate in the same batch")
	}
}
