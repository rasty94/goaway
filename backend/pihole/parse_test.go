package pihole

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func buildArchive(t *testing.T) []byte {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "gravity.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE "group" (id INTEGER PRIMARY KEY, enabled BOOLEAN, name TEXT, description TEXT)`,
		`CREATE TABLE adlist (id INTEGER PRIMARY KEY, address TEXT, enabled BOOLEAN, comment TEXT)`,
		`CREATE TABLE domainlist (id INTEGER PRIMARY KEY, type INTEGER, domain TEXT, enabled BOOLEAN, comment TEXT)`,
		`CREATE TABLE domainlist_by_group (domainlist_id INTEGER, group_id INTEGER)`,
		`CREATE TABLE client (id INTEGER PRIMARY KEY, ip TEXT, comment TEXT)`,
		`CREATE TABLE client_by_group (client_id INTEGER, group_id INTEGER)`,
		`INSERT INTO "group" VALUES (0, 1, 'Default', 'default'), (1, 1, 'Kids', 'children')`,
		`INSERT INTO adlist VALUES (1, 'https://example.org/hosts', 1, 'StevenBlack'), (2, 'https://example.org/off', 0, '')`,
		`INSERT INTO domainlist VALUES
			(1, 1, 'ads.example', 1, ''),
			(2, 0, 'ok.example', 1, ''),
			(3, 3, '(\.|^)tracker\.example$', 1, ''),
			(4, 3, '^bad', 1, ''),
			(5, 1, 'disabled.example', 0, ''),
			(6, 1, 'kids-only.example', 1, '')`,
		`INSERT INTO domainlist_by_group VALUES (1, 0), (2, 0), (3, 0), (4, 0), (6, 1)`,
		`INSERT INTO client VALUES (1, '192.168.1.20', ''), (2, 'AA:BB:CC:DD:EE:FF', ''), (3, '192.168.1.0/24', ''), (4, '192.168.1.99', '')`,
		`INSERT INTO client_by_group VALUES (1, 1), (2, 1), (3, 1), (4, 0)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	sqlDB, _ := db.DB()
	sqlDB.Close()
	dbBytes, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	tomlBytes := []byte(`
[dns]
  upstreams = [ "1.1.1.1", "127.0.0.1#5335" ]
  hosts = [ "192.168.1.1 router router.lan", "fd00::1 nas", "broken" ]
  cnameRecords = [ "media.lan,nas.lan", "nottl" ]

[dhcp]
  hosts = [ "00:11:22:33:44:55,192.168.1.50,printer,24h", "set:lab,192.168.1.60", "aa:bb:cc:dd:ee:00,id:*,192.168.1.61,infinite" ]
`)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range map[string][]byte{"etc/pihole/gravity.db": dbBytes, "etc/pihole/pihole.toml": tomlBytes} {
		w, _ := zw.Create(name)
		w.Write(content)
	}
	zw.Close()
	return buf.Bytes()
}

func TestParseTeleporter(t *testing.T) {
	plan, err := Parse(buildArchive(t))
	if err != nil {
		t.Fatal(err)
	}

	if len(plan.Adlists) != 2 || plan.Adlists[0].Name != "StevenBlack" || plan.Adlists[1].Enabled {
		t.Fatalf("adlists: %+v", plan.Adlists)
	}
	if len(plan.Blacklist) != 2 || plan.Blacklist[0].Pattern != "ads.example" || plan.Blacklist[1].Pattern != "*.tracker.example" {
		t.Fatalf("blacklist: %+v", plan.Blacklist)
	}
	if len(plan.Whitelist) != 1 || plan.Whitelist[0].Pattern != "ok.example" {
		t.Fatalf("whitelist: %+v", plan.Whitelist)
	}
	if len(plan.Groups) != 1 || plan.Groups[0].Name != "Kids" {
		t.Fatalf("groups: %+v", plan.Groups)
	}
	if len(plan.GroupDomains) != 1 || plan.GroupDomains[0] != (GroupDomain{Group: "Kids", Pattern: "kids-only.example", Blocked: true}) {
		t.Fatalf("group domains: %+v", plan.GroupDomains)
	}
	if len(plan.Clients) != 2 || plan.Clients[0].Type != "ip" || plan.Clients[1].Type != "mac" || plan.Clients[1].Identifier != "aa:bb:cc:dd:ee:ff" || plan.Clients[0].Groups[0] != "Kids" {
		t.Fatalf("clients: %+v", plan.Clients)
	}
	if len(plan.Upstreams) != 2 || plan.Upstreams[1].Address != "127.0.0.1:5335" {
		t.Fatalf("upstreams: %+v", plan.Upstreams)
	}
	wantRes := []Resolution{
		{Domain: "router", Value: "192.168.1.1", Type: "A"},
		{Domain: "router.lan", Value: "192.168.1.1", Type: "A"},
		{Domain: "nas", Value: "fd00::1", Type: "AAAA"},
		{Domain: "media.lan", Value: "nas.lan", Type: "CNAME"},
	}
	if len(plan.Resolutions) != len(wantRes) {
		t.Fatalf("resolutions: %+v", plan.Resolutions)
	}
	for i, r := range wantRes {
		if plan.Resolutions[i] != r {
			t.Fatalf("resolution %d: got %+v want %+v", i, plan.Resolutions[i], r)
		}
	}
	if len(plan.StaticLeases) != 2 || plan.StaticLeases[0] != (StaticLease{MAC: "00:11:22:33:44:55", IP: "192.168.1.50", Hostname: "printer"}) ||
		plan.StaticLeases[1] != (StaticLease{MAC: "aa:bb:cc:dd:ee:00", IP: "192.168.1.61"}) {
		t.Fatalf("leases: %+v", plan.StaticLeases)
	}
	// ^bad regex, disabled domain, CIDR client, broken host, cname without target, dhcp host without mac
	if len(plan.Skipped) != 6 {
		t.Fatalf("skipped (%d): %v", len(plan.Skipped), plan.Skipped)
	}
}

func TestParseRejectsForeignZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("settings.json")
	w.Write([]byte("{}"))
	zw.Close()
	if _, err := Parse(buf.Bytes()); err == nil {
		t.Fatal("expected error for a non Pi-hole archive")
	}
}
