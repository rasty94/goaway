package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// registerHARoutes registers High Availability management API endpoints
func (api *API) registerHARoutes() {
	// Public HA status endpoint (for monitoring)
	api.router.GET("/api/ha/status", api.getHAStatus)

	// Protected HA configuration endpoints
	haRoutes := api.routes.Group("/ha")
	{
		haRoutes.GET("/config", api.getHAConfig)
		haRoutes.POST("/config", api.saveHAConfig)
		haRoutes.POST("/sync-now", api.triggerHASync)
	}
}

// getHAConfig returns current HA configuration
//
//	@Summary Get HA Configuration
//	@Description Retrieve High Availability configuration
//	@Tags ha
//	@Success 200
//	@Router /ha/config [get]
func (api *API) getHAConfig(c *gin.Context) {
	ha := api.Config.HighAvailability
	c.JSON(http.StatusOK, gin.H{
		"enabled":               ha.Enabled,
		"mode":                  ha.Mode,
		"replicaSyncInterval":   ha.ReplicaSyncInterval,
		"primaryBackupProvider": ha.PrimaryBackupProvider,
		"primaryBackupEndpoint": ha.PrimaryBackupEndpoint,
		"primaryBackupBucket":   ha.PrimaryBackupBucket,
		"primaryBackupRegion":   ha.PrimaryBackupRegion,
		"primaryBackupUsername": ha.PrimaryBackupUsername,
	})
}

// saveHAConfig saves or updates HA configuration
//
//	@Summary Save HA Configuration
//	@Description Update High Availability settings
//	@Tags ha
//	@Accept  application/json
//	@Success 200
//	@Router /ha/config [post]
func (api *API) saveHAConfig(c *gin.Context) {
	type HAInput struct {
		Enabled                bool   `json:"enabled"`
		Mode                   string `json:"mode"` // "primary" or "replica"
		ReplicaSyncInterval    string `json:"replicaSyncInterval"`
		PrimaryBackupProvider  string `json:"primaryBackupProvider"`
		PrimaryBackupEndpoint  string `json:"primaryBackupEndpoint"`
		PrimaryBackupBucket    string `json:"primaryBackupBucket"`
		PrimaryBackupRegion    string `json:"primaryBackupRegion"`
		PrimaryBackupAccessKey string `json:"primaryBackupAccessKey"`
		PrimaryBackupSecretKey string `json:"primaryBackupSecretKey"`
		PrimaryBackupUsername  string `json:"primaryBackupUsername"`
		PrimaryBackupPassword  string `json:"primaryBackupPassword"`
	}

	var input HAInput
	if err := c.BindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid HA configuration"})
		return
	}

	// Validate mode
	mode := strings.ToLower(strings.TrimSpace(input.Mode))
	if mode != "primary" && mode != "replica" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mode must be 'primary' or 'replica'"})
		return
	}

	// Validate provider if replica mode
	if mode == "replica" && input.Enabled {
		provider := strings.ToLower(strings.TrimSpace(input.PrimaryBackupProvider))
		if provider != "s3" && provider != "webdav" && provider != "local" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid primary backup provider"})
			return
		}
	}

	ha := &api.Config.HighAvailability
	ha.Enabled = input.Enabled
	ha.Mode = mode
	ha.ReplicaSyncInterval = strings.TrimSpace(input.ReplicaSyncInterval)
	if ha.ReplicaSyncInterval == "" {
		ha.ReplicaSyncInterval = "15m"
	}
	ha.PrimaryBackupProvider = strings.ToLower(strings.TrimSpace(input.PrimaryBackupProvider))
	ha.PrimaryBackupEndpoint = strings.TrimSpace(input.PrimaryBackupEndpoint)
	ha.PrimaryBackupBucket = strings.TrimSpace(input.PrimaryBackupBucket)
	ha.PrimaryBackupRegion = strings.TrimSpace(input.PrimaryBackupRegion)
	ha.PrimaryBackupUsername = strings.TrimSpace(input.PrimaryBackupUsername)

	// Only overwrite secrets if provided
	if input.PrimaryBackupAccessKey != "" {
		ha.PrimaryBackupAccessKey = input.PrimaryBackupAccessKey
	}
	if input.PrimaryBackupSecretKey != "" {
		ha.PrimaryBackupSecretKey = input.PrimaryBackupSecretKey
	}
	if input.PrimaryBackupPassword != "" {
		ha.PrimaryBackupPassword = input.PrimaryBackupPassword
	}

	api.Config.Save()
	c.JSON(http.StatusOK, gin.H{"message": "HA configuration saved"})
}

// getHAStatus returns current HA status
//
//	@Summary Get HA Status
//	@Description Get High Availability status including last sync time
//	@Tags ha
//	@Success 200
//	@Router /ha/status [get]
func (api *API) getHAStatus(c *gin.Context) {
	ha := api.Config.HighAvailability
	c.JSON(http.StatusOK, gin.H{
		"enabled":      ha.Enabled,
		"mode":         ha.Mode,
		"configured":   ha.PrimaryBackupProvider != "",
		"lastSyncTime": ha.LastSyncTime,
	})
}

// triggerHASync manually triggers a sync cycle for replica instances
//
//	@Summary Trigger HA Sync
//	@Description Manually trigger a synchronization from Primary to Replica
//	@Tags ha
//	@Success 200
//	@Router /ha/sync-now [post]
func (api *API) triggerHASync(c *gin.Context) {
	ha := api.Config.HighAvailability
	if !ha.Enabled || ha.Mode != "replica" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "manual sync only available in replica mode"})
		return
	}

	// Check if replica sync manager exists
	if api.ReplicaSyncManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "replica sync manager not initialized"})
		return
	}

	// Trigger sync in background
	go func() {
		if err := api.ReplicaSyncManager.SyncNow(); err != nil {
			log.Error("Manual HA sync triggered from API failed: %v", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{"message": "HA sync triggered"})
}
