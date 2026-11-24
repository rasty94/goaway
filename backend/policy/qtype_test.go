package policy

import (
	"goaway/backend/database"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchesQueryType(t *testing.T) {
	assert.True(t, MatchesQueryType("AAAA,HTTPS,SVCB", "aaaa"))
	assert.True(t, MatchesQueryType(" AAAA , https ", "HTTPS"))
	assert.False(t, MatchesQueryType("AAAA,HTTPS", "A"))
	assert.False(t, MatchesQueryType("", "AAAA"))
	assert.False(t, MatchesQueryType("AAAA", ""))
}

func TestShouldBlockQueryType(t *testing.T) {
	svc := &Service{
		policies: []database.Policy{
			{
				ID:                1,
				Name:              "block-https-v6",
				Enabled:           true,
				BlockedQueryTypes: "AAAA,HTTPS",
			},
			{
				ID:                2,
				Name:              "dryrun-svcb",
				Enabled:           true,
				IsDryRun:          true,
				BlockedQueryTypes: "SVCB",
			},
		},
		assignments: map[string][]uint{
			"ip:10.0.0.5": {1},
			"group:7":     {2},
		},
	}

	blocked, name, dryRun := svc.ShouldBlockQueryType("10.0.0.5", "", nil, "HTTPS")
	assert.True(t, blocked)
	assert.Equal(t, "block-https-v6", name)
	assert.False(t, dryRun)

	blocked, _, _ = svc.ShouldBlockQueryType("10.0.0.5", "", nil, "A")
	assert.False(t, blocked)

	blocked, name, dryRun = svc.ShouldBlockQueryType("10.0.0.9", "", []uint{7}, "SVCB")
	assert.True(t, blocked)
	assert.Equal(t, "dryrun-svcb", name)
	assert.True(t, dryRun)
}
