package api

import (
	"context"
	"goaway/backend/database"
	"goaway/backend/dhcp"
	"goaway/backend/resolution"
	"goaway/backend/settings"
	"goaway/backend/whitelist"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newSnapshotTestAPI(t *testing.T, secret string) *API {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&database.Whitelist{},
		&database.Resolution{},
		&database.StaticDHCPLease{},
	))

	cfg := &settings.Config{
		HighAvailability: settings.HighAvailabilityConfig{
			Enabled: true,
			Secret:  secret,
		},
	}

	a := &API{
		Config:            cfg,
		router:            gin.New(),
		WhitelistService:  whitelist.NewService(whitelist.NewRepository(db)),
		ResolutionService: resolution.NewService(resolution.NewRepository(db)),
		DHCPService:       dhcp.NewService(dhcp.NewRepository(db), cfg),
	}
	a.routes = a.router.Group("/api")
	a.registerHARoutes()
	return a
}

func TestClusterLogicalHotResyncBetweenNodes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "ha-snapshot-secret"

	primary := newSnapshotTestAPI(t, secret)
	require.NoError(t, primary.WhitelistService.AddDomain("safe.example.com"))
	require.NoError(t, primary.ResolutionService.CreateResolution("192.168.1.10", "nas.lan", "A"))
	require.NoError(t, primary.DHCPService.CreateStaticLease(&database.StaticDHCPLease{
		MAC:      "aa:bb:cc:dd:ee:ff",
		IP:       "192.168.1.50",
		Hostname: "printer",
		Enabled:  true,
	}))

	primarySrv := httptest.NewServer(primary.router)
	t.Cleanup(primarySrv.Close)

	replica := newSnapshotTestAPI(t, secret)
	snap, err := replica.fetchClusterSnapshot(context.Background(), primarySrv.URL)
	require.NoError(t, err)

	rep := replica.applyClusterSnapshot(snap)
	assert.Equal(t, 1, rep.WhitelistAdded)
	assert.Equal(t, 1, rep.ResolutionsAdded)
	assert.Equal(t, 1, rep.StaticLeasesAdded)

	assert.True(t, replica.WhitelistService.IsWhitelisted("safe.example.com"))
	res, err := replica.ResolutionService.GetResolution("nas.lan")
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.10", res.Value)
	leases, err := replica.DHCPService.ListStaticLeases()
	require.NoError(t, err)
	require.Len(t, leases, 1)

	// Re-applying the same snapshot is idempotent (0 new items)
	rep2 := replica.applyClusterSnapshot(snap)
	assert.Equal(t, resyncReport{}, rep2)
}
