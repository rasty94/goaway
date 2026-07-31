package group

import (
	"goaway/backend/database"
	"goaway/backend/domain"
	"testing"
)

// serviceWithGroup builds a Service with a pre-populated cache so the blocking
// decision can be exercised without a database.
func serviceWithGroup(t *testing.T, useGlobal bool, blocked, allowed []string) *Service {
	t.Helper()

	const groupID = uint(1)

	blockMatcher := domain.NewMatcher()
	for _, d := range blocked {
		blockMatcher.Add(d)
	}
	allowMatcher := domain.NewMatcher()
	for _, d := range allowed {
		allowMatcher.Add(d)
	}

	return &Service{
		groupsByID: map[uint]database.ClientGroup{
			groupID: {ID: groupID, Name: "kids", UseGlobalPolicies: useGlobal},
		},
		assignments:    map[string][]uint{identifierKey("ip", "10.0.0.5"): {groupID}},
		blockedByGroup: map[uint]*domain.Matcher{groupID: blockMatcher},
		allowedByGroup: map[uint]*domain.Matcher{groupID: allowMatcher},
	}
}

func TestShouldBlockDetailedPrecedence(t *testing.T) {
	tests := []struct {
		name              string
		ip                string
		domainName        string
		useGlobal         bool
		blocked           []string
		allowed           []string
		globalBlocked     bool
		globalWhitelisted bool
		want              bool
		wantSource        string
	}{
		{
			name:       "group blocklist blocks",
			ip:         "10.0.0.5",
			domainName: "ads.example.com",
			blocked:    []string{"ads.example.com"},
			want:       true,
			wantSource: "Group Blocked",
		},
		{
			name:       "group allowlist beats group blocklist",
			ip:         "10.0.0.5",
			domainName: "ads.example.com",
			blocked:    []string{"ads.example.com"},
			allowed:    []string{"ads.example.com"},
			want:       false,
			wantSource: "Group Allowed",
		},
		{
			name:              "global whitelist beats everything",
			ip:                "10.0.0.5",
			domainName:        "ads.example.com",
			blocked:           []string{"ads.example.com"},
			globalWhitelisted: true,
			want:              false,
			wantSource:        "Global Whitelist",
		},
		{
			name:          "global blacklist applies when group opts in",
			ip:            "10.0.0.5",
			domainName:    "tracker.example.com",
			useGlobal:     true,
			globalBlocked: true,
			want:          true,
			wantSource:    "Global Blacklist",
		},
		{
			name:          "global blacklist ignored when group opts out",
			ip:            "10.0.0.5",
			domainName:    "tracker.example.com",
			useGlobal:     false,
			globalBlocked: true,
			want:          false,
		},
		{
			name:          "unassigned client is not covered by group rules",
			ip:            "10.0.0.99",
			domainName:    "ads.example.com",
			blocked:       []string{"ads.example.com"},
			globalBlocked: true,
			want:          false,
		},
		{
			name:       "group wildcard blocks subdomain",
			ip:         "10.0.0.5",
			domainName: "cdn.ads.example.com",
			blocked:    []string{"*.ads.example.com"},
			want:       true,
			wantSource: "Group Blocked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := serviceWithGroup(t, tt.useGlobal, tt.blocked, tt.allowed)

			got, source, _ := s.ShouldBlockDetailed(
				tt.ip, "", tt.domainName, tt.domainName,
				tt.globalBlocked, tt.globalWhitelisted,
			)

			if got != tt.want {
				t.Errorf("blocked = %v, want %v (source %q)", got, tt.want, source)
			}
			if tt.wantSource != "" && source != tt.wantSource {
				t.Errorf("source = %q, want %q", source, tt.wantSource)
			}
		})
	}
}

func TestNormalizeDomainIsCaseAndDotInsensitive(t *testing.T) {
	for _, in := range []string{"Example.COM.", "example.com", "EXAMPLE.com."} {
		if got := normalizeDomain(in); got != "example.com" {
			t.Errorf("normalizeDomain(%q) = %q, want %q", in, got, "example.com")
		}
	}
}
