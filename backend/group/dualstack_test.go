package group

import (
	"goaway/backend/database"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestDualStackPolicyConsistencyViaMACAndHostname(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&database.ClientGroup{},
		&database.ClientGroupAssignment{},
		&database.GroupBlockedDomain{},
		&database.GroupAllowedDomain{},
	))

	svc := NewService(NewRepository(db))
	kidsGroup, err := svc.CreateGroup("kids", "Kids filter", true)
	require.NoError(t, err)
	_, err = svc.AddBlockedDomains(kidsGroup.ID, []string{"games.example.com"})
	require.NoError(t, err)

	// Assign only by IPv4 address
	require.NoError(t, svc.ReplaceAssignments("192.168.1.20", IdentifierTypeIP, []uint{kidsGroup.ID}))

	// Correlate IPv4 and IPv6 via shared MAC
	svc.RecordClientIdentity("192.168.1.20", "aa:bb:cc:dd:ee:ff", "tablet.lan")
	svc.RecordClientIdentity("fd00::20", "aa:bb:cc:dd:ee:ff", "unknown")

	polV6 := svc.GetEffectivePolicy("fd00::20", "aa:bb:cc:dd:ee:ff")
	assert.Contains(t, polV6.GroupNames, "kids")
	assert.True(t, svc.ShouldBlock("fd00::20", "aa:bb:cc:dd:ee:ff", "games.example.com", "games.example.com", false, false))

	// Correlate a second IPv6 address via shared hostname even when MAC is unknown
	svc.RecordClientIdentity("2001:db8::20", "unknown", "tablet.lan")
	polHost := svc.GetEffectivePolicy("2001:db8::20", "unknown")
	assert.Contains(t, polHost.GroupNames, "kids")
	assert.True(t, svc.ShouldBlock("2001:db8::20", "unknown", "games.example.com", "games.example.com", false, false))
}
