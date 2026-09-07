// Package pihole reads a Pi-hole v6 Teleporter archive and turns it into an import plan.
package pihole

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/glebarez/sqlite"
	"github.com/pelletier/go-toml/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Adlist struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
	Exists  bool   `json:"exists"`
}

type Domain struct {
	Pattern string `json:"pattern"`
	Exists  bool   `json:"exists"`
}

type Group struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Exists      bool   `json:"exists"`
}

type GroupDomain struct {
	Group   string `json:"group"`
	Pattern string `json:"pattern"`
	Blocked bool   `json:"blocked"`
	Exists  bool   `json:"exists"`
}

type Client struct {
	Identifier string   `json:"identifier"`
	Type       string   `json:"type"` // ip | mac
	Groups     []string `json:"groups"`
	Exists     bool     `json:"exists"`
}

type Resolution struct {
	Domain string `json:"domain"`
	Value  string `json:"value"`
	Type   string `json:"type"`
	Exists bool   `json:"exists"`
}

type Upstream struct {
	Address string `json:"address"`
	Exists  bool   `json:"exists"`
}

type StaticLease struct {
	MAC      string `json:"mac"`
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	Exists   bool   `json:"exists"`
}

// Plan is everything found in the archive, ready to be reviewed and applied.
type Plan struct {
	Adlists      []Adlist      `json:"adlists"`
	Blacklist    []Domain      `json:"blacklist"`
	Whitelist    []Domain      `json:"whitelist"`
	Groups       []Group       `json:"groups"`
	GroupDomains []GroupDomain `json:"groupDomains"`
	Clients      []Client      `json:"clients"`
	Resolutions  []Resolution  `json:"resolutions"`
	Upstreams    []Upstream    `json:"upstreams"`
	StaticLeases []StaticLease `json:"staticLeases"`
	Skipped      []string      `json:"skipped"`
}

func (p *Plan) skip(format string, args ...interface{}) {
	p.Skipped = append(p.Skipped, fmt.Sprintf(format, args...))
}

// Parse reads a Teleporter zip (Pi-hole v6) from memory.
func Parse(data []byte) (*Plan, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("not a zip archive: %w", err)
	}

	plan := &Plan{
		Adlists: []Adlist{}, Blacklist: []Domain{}, Whitelist: []Domain{}, Groups: []Group{},
		GroupDomains: []GroupDomain{}, Clients: []Client{}, Resolutions: []Resolution{},
		Upstreams: []Upstream{}, StaticLeases: []StaticLease{}, Skipped: []string{},
	}
	found := false
	for _, f := range zr.File {
		switch path.Base(f.Name) {
		case "gravity.db":
			content, err := readEntry(f)
			if err != nil {
				return nil, err
			}
			if err := plan.readGravity(content); err != nil {
				return nil, err
			}
			found = true
		case "pihole.toml":
			content, err := readEntry(f)
			if err != nil {
				return nil, err
			}
			if err := plan.readToml(content); err != nil {
				return nil, err
			}
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("archive contains neither gravity.db nor pihole.toml; only Pi-hole v6 Teleporter exports are supported")
	}
	return plan, nil
}

func readEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("cannot open %s: %w", f.Name, err)
	}
	defer rc.Close()
	// ponytail: 256 MB cap; a gravity.db bigger than that is all downloaded list rows we do not read anyway
	return io.ReadAll(io.LimitReader(rc, 256<<20))
}

// --- gravity.db ---

type gravityGroup struct {
	ID          int
	Name        string
	Enabled     bool
	Description string
}

func (p *Plan) readGravity(content []byte) error {
	tmp, err := os.CreateTemp("", "pihole-gravity-*.db")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	db, err := gorm.Open(sqlite.Open("file:"+filepath.ToSlash(tmpPath)+"?mode=ro"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return fmt.Errorf("gravity.db is not a valid SQLite database: %w", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	var groups []gravityGroup
	if err := db.Raw(`SELECT id, name, enabled, description FROM "group"`).Scan(&groups).Error; err != nil {
		return fmt.Errorf("gravity.db: %w", err)
	}
	groupName := map[int]string{}
	for _, g := range groups {
		groupName[g.ID] = g.Name
		if g.ID == 0 {
			continue // Pi-hole's Default maps onto GoAway's default group
		}
		p.Groups = append(p.Groups, Group{Name: g.Name, Description: g.Description})
	}

	var adlists []struct {
		Address string
		Enabled bool
		Comment string
	}
	if err := db.Raw(`SELECT address, enabled, comment FROM adlist`).Scan(&adlists).Error; err != nil {
		return fmt.Errorf("gravity.db: %w", err)
	}
	// GoAway toggles sources by name, so names must be unique: fall back to the URL on blanks and duplicates.
	nameCount := map[string]int{}
	for _, a := range adlists {
		nameCount[strings.TrimSpace(a.Comment)]++
	}
	for _, a := range adlists {
		name := strings.TrimSpace(a.Comment)
		if name == "" || nameCount[name] > 1 {
			name = a.Address
		}
		p.Adlists = append(p.Adlists, Adlist{Name: name, URL: a.Address, Enabled: a.Enabled})
	}

	var links []struct {
		DomainlistID int
		GroupID      int
	}
	if err := db.Raw(`SELECT domainlist_id, group_id FROM domainlist_by_group`).Scan(&links).Error; err != nil {
		return fmt.Errorf("gravity.db: %w", err)
	}
	domainGroups := map[int][]int{}
	for _, l := range links {
		domainGroups[l.DomainlistID] = append(domainGroups[l.DomainlistID], l.GroupID)
	}

	var domains []struct {
		ID      int
		Type    int
		Domain  string
		Enabled bool
	}
	if err := db.Raw(`SELECT id, type, domain, enabled FROM domainlist`).Scan(&domains).Error; err != nil {
		return fmt.Errorf("gravity.db: %w", err)
	}
	for _, d := range domains {
		if !d.Enabled {
			p.skip("domain %q is disabled in Pi-hole", d.Domain)
			continue
		}
		allow := d.Type == 0 || d.Type == 2
		pattern := d.Domain
		if d.Type >= 2 {
			pattern = convertRegex(d.Domain)
		}
		global := false
		for _, gid := range domainGroups[d.ID] {
			if gid == 0 {
				global = true
				continue
			}
			p.GroupDomains = append(p.GroupDomains, GroupDomain{Group: groupName[gid], Pattern: pattern, Blocked: !allow})
		}
		if len(domainGroups[d.ID]) == 0 {
			global = true
		}
		if !global {
			continue
		}
		if allow {
			p.Whitelist = append(p.Whitelist, Domain{Pattern: pattern})
		} else if strings.HasPrefix(pattern, "/") {
			p.skip("global regex deny rule %q has no equivalent; add it to a group list instead", d.Domain)
		} else {
			p.Blacklist = append(p.Blacklist, Domain{Pattern: pattern})
		}
	}

	var clients []struct {
		ID int
		IP string
	}
	if err := db.Raw(`SELECT id, ip FROM client`).Scan(&clients).Error; err != nil {
		return fmt.Errorf("gravity.db: %w", err)
	}
	var clientLinks []struct {
		ClientID int
		GroupID  int
	}
	if err := db.Raw(`SELECT client_id, group_id FROM client_by_group`).Scan(&clientLinks).Error; err != nil {
		return fmt.Errorf("gravity.db: %w", err)
	}
	clientGroups := map[int][]string{}
	for _, l := range clientLinks {
		if l.GroupID != 0 {
			clientGroups[l.ClientID] = append(clientGroups[l.ClientID], groupName[l.GroupID])
		}
	}
	for _, c := range clients {
		id := strings.TrimSpace(c.IP)
		var kind string
		switch {
		case net.ParseIP(id) != nil:
			kind = "ip"
		case isMAC(id):
			kind = "mac"
		default:
			p.skip("client %q is not a plain IP or MAC (subnets, hostnames and interfaces are not supported)", id)
			continue
		}
		groups := clientGroups[c.ID]
		if len(groups) == 0 {
			continue // only in Default: nothing to assign
		}
		p.Clients = append(p.Clients, Client{Identifier: strings.ToLower(id), Type: kind, Groups: groups})
	}
	return nil
}

func isMAC(s string) bool {
	_, err := net.ParseMAC(s)
	return err == nil && len(s) == 17
}

var (
	// wildcardRegex matches the regex Pi-hole writes for "add as wildcard": (\.|^)example\.com$
	wildcardRegex = regexp.MustCompile(`^\(\\\.\|\^\)(.+)\$$`)
	plainDomain   = regexp.MustCompile(`^[a-z0-9.-]+$`)
)

// convertRegex maps a Pi-hole regex onto GoAway syntax: *.domain for the wildcard form, /re/ otherwise.
func convertRegex(re string) string {
	if m := wildcardRegex.FindStringSubmatch(re); m != nil {
		domain := strings.ToLower(strings.ReplaceAll(m[1], `\.`, "."))
		if plainDomain.MatchString(domain) {
			return "*." + domain
		}
	}
	return "/" + re + "/"
}

// --- pihole.toml ---

type tomlConfig struct {
	DNS struct {
		Upstreams    []string `toml:"upstreams"`
		Hosts        []string `toml:"hosts"`
		CnameRecords []string `toml:"cnameRecords"`
	} `toml:"dns"`
	DHCP struct {
		Hosts []string `toml:"hosts"`
	} `toml:"dhcp"`
}

func (p *Plan) readToml(content []byte) error {
	var cfg tomlConfig
	if err := toml.Unmarshal(content, &cfg); err != nil {
		return fmt.Errorf("pihole.toml: %w", err)
	}

	for _, u := range cfg.DNS.Upstreams {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		// Pi-hole writes host#port (port optional); GoAway dials host:port, IPv6 in brackets
		host, port := u, "53"
		if i := strings.LastIndex(u, "#"); i > 0 {
			host, port = u[:i], u[i+1:]
		}
		p.Upstreams = append(p.Upstreams, Upstream{Address: net.JoinHostPort(host, port)})
	}

	for _, h := range cfg.DNS.Hosts {
		fields := strings.Fields(h)
		if len(fields) < 2 || net.ParseIP(fields[0]) == nil {
			p.skip("host entry %q is not '<ip> <name...>'", h)
			continue
		}
		recType := "A"
		if net.ParseIP(fields[0]).To4() == nil {
			recType = "AAAA"
		}
		for _, name := range fields[1:] {
			p.Resolutions = append(p.Resolutions, Resolution{Domain: strings.ToLower(name), Value: fields[0], Type: recType})
		}
	}

	for _, c := range cfg.DNS.CnameRecords {
		parts := strings.Split(c, ",")
		if len(parts) < 2 {
			p.skip("cname entry %q is not '<name>,<target>'", c)
			continue
		}
		p.Resolutions = append(p.Resolutions, Resolution{
			Domain: strings.ToLower(strings.TrimSpace(parts[0])),
			Value:  strings.ToLower(strings.TrimSpace(parts[1])),
			Type:   "CNAME",
		})
	}

	for _, h := range cfg.DHCP.Hosts {
		lease, ok := parseDHCPHost(h)
		if !ok {
			p.skip("dhcp host %q has no IPv4 address and MAC", h)
			continue
		}
		p.StaticLeases = append(p.StaticLeases, lease)
	}
	return nil
}

// parseDHCPHost reads dnsmasq's "mac,ip,hostname,leasetime" where every field is optional and may carry set:/id:/tag: prefixes.
func parseDHCPHost(h string) (StaticLease, bool) {
	var lease StaticLease
	for _, f := range strings.Split(h, ",") {
		f = strings.TrimSpace(f)
		switch {
		case f == "" || strings.Contains(f, ":") && !isMAC(f) && net.ParseIP(f) == nil:
			// set:tag, id:*, tag:x, ignore, or an IPv6 address we do not support
		case lease.MAC == "" && isMAC(f):
			lease.MAC = strings.ToLower(f)
		case lease.IP == "" && net.ParseIP(f) != nil && net.ParseIP(f).To4() != nil:
			lease.IP = f
		case leaseTime.MatchString(f):
		case lease.Hostname == "":
			lease.Hostname = f
		}
	}
	return lease, lease.MAC != "" && lease.IP != ""
}

var leaseTime = regexp.MustCompile(`^(\d+[smhdw]|infinite)$`)
