package api

import (
	"context"
	"fmt"
	"goaway/backend/cluster"
	"net/http"
	"strings"
	"time"

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
		haRoutes.GET("/cluster", api.getClusterStatus)
	}

	// Internal clustering heartbeat (no auth required if verified separately or over TLS)
	api.router.POST("/api/native/cluster/heartbeat", api.handleHeartbeat)
	api.router.POST("/api/native/cluster/replicate", api.handleReplication)
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

func (api *API) handleHeartbeat(c *gin.Context) {
	var req cluster.HeartbeatRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid heartbeat request"})
		return
	}

	// In a real scenario, we would verify the SourceID and Timestamp/Signature
	// For now, we just reply with our current status
	resp := cluster.HeartbeatResponse{
		ID:        "local-node", // Should match initialized ID
		Role:      cluster.NodeRole(api.Config.HighAvailability.Mode),
		Timestamp: time.Now(),
		Status:    "online",
	}

	c.JSON(http.StatusOK, resp)
}

func (api *API) getClusterStatus(c *gin.Context) {
	if api.ClusterManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cluster manager not initialized"})
		return
	}

	nodes := api.ClusterManager.GetNodes()
	count := 0
	for _, n := range nodes {
		if !n.Unreachable {
			count++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"selfRole":    api.Config.HighAvailability.Mode,
		"activeNodes": count,
		"clusterId":   "goaway-cluster-main", // Static for now, or fetch from config
		"nodes":       nodes,
	})
}

func (api *API) handleReplication(c *gin.Context) {
	var event cluster.ReplicatedEvent
	if err := c.BindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid replication event"})
		return
	}

	log.Info("[HA/Replication] Received event: %s", event.Type)

	// Dispatch event to appropriate service
	var err error
	switch event.Type {
	case cluster.EventBlacklistAdd:
		err = api.handleBlacklistReplication(event.Payload)
	case cluster.EventBlacklistRemove:
		err = api.handleBlacklistRemoveReplication(event.Payload)
	case cluster.EventWhitelistAdd:
		err = api.handleWhitelistReplication(event.Payload)
	case cluster.EventWhitelistRemove:
		err = api.handleWhitelistRemoveReplication(event.Payload)
	case cluster.EventGroupCreate:
		err = api.handleGroupCreateReplication(event.Payload)
	case cluster.EventGroupDelete:
		err = api.handleGroupDeleteReplication(event.Payload)
	// Add other cases as needed
	default:
		log.Warning("[HA/Replication] Unhandled event type: %s", event.Type)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func (api *API) handleBlacklistReplication(payload interface{}) error {
	// Payload is likely map[string]interface{} from JSON
	m, ok := payload.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid blacklist payload")
	}

	domainStr, _ := m["domain"].(string)
	if domainStr == "" {
		return fmt.Errorf("empty domain in replication")
	}

	// Add without re-broadcasting
	return api.BlacklistService.AddBlacklistedDomain(context.Background(), domainStr)
}

func (api *API) handleBlacklistRemoveReplication(payload interface{}) error {
	m, ok := payload.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid blacklist remove payload")
	}

	domainStr, _ := m["domain"].(string)
	if domainStr == "" {
		return fmt.Errorf("empty domain in replication")
	}

	return api.BlacklistService.RemoveDomain(context.Background(), domainStr)
}

func (api *API) handleWhitelistReplication(payload interface{}) error {
	m, ok := payload.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid whitelist payload")
	}

	domainStr, _ := m["domain"].(string)
	if domainStr == "" {
		return fmt.Errorf("empty domain in replication")
	}

	return api.WhitelistService.AddDomain(domainStr)
}

func (api *API) handleWhitelistRemoveReplication(payload interface{}) error {
	m, ok := payload.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid whitelist remove payload")
	}

	domainStr, _ := m["domain"].(string)
	if domainStr == "" {
		return fmt.Errorf("empty domain in replication")
	}

	return api.WhitelistService.RemoveDomain(domainStr)
}

func (api *API) handleGroupCreateReplication(payload interface{}) error {
	// For Group creation, we can try to JSON decode it back to database.ClientGroup
	// But let's keep it simple for now and just extra fields
	m, ok := payload.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid group payload")
	}

	name, _ := m["name"].(string)
	desc, _ := m["description"].(string)
	global, _ := m["useGlobalPolicies"].(bool)

	if name == "" {
		return fmt.Errorf("empty group name in replication")
	}

	_, err := api.GroupService.CreateGroup(name, desc, global)
	return err
}

func (api *API) handleGroupDeleteReplication(payload interface{}) error {
	m, ok := payload.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid group delete payload")
	}

	// JSON numbers are often float64
	idVal, _ := m["id"].(float64)
	if idVal == 0 {
		return fmt.Errorf("missing id in group delete")
	}

	return api.GroupService.DeleteGroup(uint(idVal))
}
