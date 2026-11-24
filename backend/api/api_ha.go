package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"goaway/backend/audit"
	"goaway/backend/cluster"
	"goaway/backend/database"
	"io"
	"net/http"
	"strings"
	"sync"
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
		haRoutes.POST("/resync", api.triggerLogicalResync)
		haRoutes.GET("/cluster", api.getClusterStatus)
	}

	// Internal node-to-node endpoints: HMAC-signed with highAvailability.secret, never the user session.
	native := api.router.Group("/api/native/cluster", api.clusterAuth())
	native.POST("/heartbeat", api.handleHeartbeat)
	native.POST("/replicate", api.handleReplication)
	native.GET("/snapshot", api.handleClusterSnapshot)
}

const clusterAuditWindow = time.Minute

// auditThrottle keeps a flood of rejected cluster requests from filling the audit table.
type auditThrottle struct {
	mu   sync.Mutex
	last time.Time
}

func (t *auditThrottle) allow() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if time.Since(t.last) < clusterAuditWindow {
		return false
	}
	t.last = time.Now()
	return true
}

// clusterAuth verifies the peer signature over timestamp+path+body and fails closed when no secret is configured.
func (api *API) clusterAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "unreadable body"})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		if err := cluster.VerifyRequestWithReplay(c.Request.Header, api.Config.HighAvailability.Secret, c.Request.URL.Path, body, time.Now(), &api.clusterNonces); err != nil {
			msg := fmt.Sprintf("Rejected cluster request from %s to %s: %v", c.ClientIP(), c.Request.URL.Path, err)
			log.Warning("[HA/Cluster] %s", msg)
			// These routes are unauthenticated by definition, so one audit row per request
			// would let anyone fill the table. Record at most one per window.
			if api.AuditService != nil && api.clusterAuthAudit.allow() {
				api.AuditService.CreateAudit(&audit.Entry{Topic: audit.TopicAPI, Message: msg, CreatedAt: time.Now()})
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid cluster signature"})
			return
		}
		c.Next()
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

func (api *API) handleHeartbeat(c *gin.Context) {
	var req cluster.HeartbeatRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid heartbeat request"})
		return
	}

	// Signature already verified by clusterAuth; reply with our current identity and role
	resp := cluster.HeartbeatResponse{
		ID:        "local-node",
		Role:      cluster.NodeRole(api.Config.HighAvailability.Mode),
		Priority:  api.Config.HighAvailability.Priority,
		Timestamp: time.Now(),
		Status:    "online",
	}
	if api.ClusterManager != nil {
		resp.ID = api.ClusterManager.SelfID()
		if api.ClusterManager.IsPrimary() {
			resp.Role = cluster.RolePrimary
		} else {
			resp.Role = cluster.RoleReplica
		}
	}

	payload, err := json.Marshal(resp)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encode heartbeat response"})
		return
	}
	cluster.SignHeaders(c.Writer.Header(), api.Config.HighAvailability.Secret, c.Request.URL.Path, payload)
	c.Data(http.StatusOK, "application/json", payload)
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

	var proxyStats map[string]interface{}
	if api.DNSProxy != nil {
		proxyStats = api.DNSProxy.GetStats()
	}

	c.JSON(http.StatusOK, gin.H{
		"selfRole":    api.Config.HighAvailability.Mode,
		"activeNodes": count,
		"clusterId":   "goaway-cluster-main", // Static for now, or fetch from config
		"nodes":       nodes,
		"proxyStats":  proxyStats,
	})
}

func (api *API) handleReplication(c *gin.Context) {
	var event cluster.ReplicatedEvent
	if err := c.BindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid replication event"})
		return
	}

	// A Primary never applies foreign events: doing so would re-broadcast them and ping-pong forever.
	if api.ClusterManager != nil && api.ClusterManager.IsPrimary() {
		c.JSON(http.StatusConflict, gin.H{"error": "this node is primary; refusing replicated event"})
		return
	}

	entityKey := cluster.EventEntityKey(event)
	if !api.clusterVersions.ShouldApply(entityKey, event.Version) {
		log.Info("[HA/Replication] Discarding stale event %s (key=%s, version=%d)", event.Type, entityKey, event.Version)
		c.Status(http.StatusOK)
		return
	}

	log.Info("[HA/Replication] Received event: %s", event.Type)

	// Dispatch event to appropriate service
	var err error
	switch event.Type {
	case cluster.EventBlacklistAdd:
		err = api.handleBlacklistReplication(event.Payload)
	case cluster.EventBlacklistAddBatch:
		err = api.handleBlacklistAddBatchReplication(event.Payload)
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
	case cluster.EventGroupUpdate:
		err = api.handleGroupUpdateReplication(event.Payload)
	case cluster.EventGroupBlockedAdd, cluster.EventGroupBlockedRemove, cluster.EventGroupAllowedAdd, cluster.EventGroupAllowedRemove:
		err = api.handleGroupDomainReplication(event.Type, event.Payload)
	case cluster.EventClientUpdate:
		err = api.handleClientUpdateReplication(event.Payload)
	case cluster.EventResolutionAdd:
		err = api.handleResolutionAddReplication(event.Payload)
	case cluster.EventResolutionRemove:
		err = api.handleResolutionRemoveReplication(event.Payload)
	case cluster.EventDHCPLeaseAdd:
		err = api.handleDHCPLeaseReplication(event.Payload)
	case cluster.EventDHCPLeasev6Add:
		err = api.handleDHCPLeasev6Replication(event.Payload)
	case cluster.EventDHCPStaticAdd:
		err = api.handleDHCPStaticAddReplication(event.Payload)
	case cluster.EventDHCPStaticRemove:
		err = api.handleDHCPStaticRemoveReplication(event.Payload)
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

// handleBlacklistAddBatchReplication applies a batch add on a replica. AddCustomDomains errors on
// the first domain that is already blacklisted, so redelivery would otherwise fail on domains this
// replica already applied; filter those out first so a retried batch is a no-op instead of an error.
func (api *API) handleBlacklistAddBatchReplication(payload interface{}) error {
	var p struct {
		Domains []string `json:"domains"`
	}
	if err := decodePayload(payload, &p); err != nil {
		return fmt.Errorf("invalid blacklist batch payload")
	}

	remaining := make([]string, 0, len(p.Domains))
	for _, d := range p.Domains {
		if !api.BlacklistService.IsBlacklisted(d) {
			remaining = append(remaining, d)
		}
	}
	if len(remaining) == 0 {
		return nil
	}

	return api.BlacklistService.AddCustomDomains(context.Background(), remaining)
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
	var p struct {
		Name              string `json:"name"`
		Description       string `json:"description"`
		UseGlobalPolicies bool   `json:"useGlobalPolicies"`
	}
	if err := decodePayload(payload, &p); err != nil || p.Name == "" {
		return fmt.Errorf("invalid group payload")
	}
	_, err := api.GroupService.CreateGroup(p.Name, p.Description, p.UseGlobalPolicies)
	return err
}

func (api *API) resolveGroupID(name string, fallbackID uint) uint {
	if name != "" && api.GroupService != nil {
		if id, ok := api.GroupService.GetGroupIDByName(name); ok {
			return id
		}
	}
	return fallbackID
}

func (api *API) handleGroupDeleteReplication(payload interface{}) error {
	var p struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	}
	if err := decodePayload(payload, &p); err != nil {
		return fmt.Errorf("invalid group delete payload")
	}
	targetID := api.resolveGroupID(p.Name, p.ID)
	if targetID == 0 {
		return fmt.Errorf("invalid group delete payload")
	}
	return api.GroupService.DeleteGroup(targetID)
}

// decodePayload re-encodes the generic JSON payload into a typed struct.
func decodePayload(payload interface{}, out interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func (api *API) handleGroupUpdateReplication(payload interface{}) error {
	var p struct {
		ID                uint   `json:"id"`
		Name              string `json:"name"`
		UseGlobalPolicies bool   `json:"useGlobalPolicies"`
	}
	if err := decodePayload(payload, &p); err != nil {
		return fmt.Errorf("invalid group update payload")
	}
	targetID := api.resolveGroupID(p.Name, p.ID)
	if targetID == 0 {
		return fmt.Errorf("invalid group update payload")
	}
	return api.GroupService.SetGroupUseGlobalPolicies(targetID, p.UseGlobalPolicies)
}

func (api *API) handleGroupDomainReplication(t cluster.EventType, payload interface{}) error {
	var p struct {
		GroupID   uint   `json:"groupId"`
		GroupName string `json:"groupName"`
		Domain    string `json:"domain"`
	}
	if err := decodePayload(payload, &p); err != nil || p.Domain == "" {
		return fmt.Errorf("invalid group domain payload")
	}
	targetID := api.resolveGroupID(p.GroupName, p.GroupID)
	if targetID == 0 {
		return fmt.Errorf("invalid group domain payload")
	}
	switch t {
	case cluster.EventGroupBlockedAdd:
		return api.GroupService.AddBlockedDomain(targetID, p.Domain)
	case cluster.EventGroupBlockedRemove:
		return api.GroupService.RemoveBlockedDomain(targetID, p.Domain)
	case cluster.EventGroupAllowedAdd:
		return api.GroupService.AddAllowedDomain(targetID, p.Domain)
	default:
		return api.GroupService.RemoveAllowedDomain(targetID, p.Domain)
	}
}

func (api *API) handleClientUpdateReplication(payload interface{}) error {
	var p struct {
		Identifier     string   `json:"identifier"`
		IdentifierType string   `json:"identifierType"`
		GroupIDs       []uint   `json:"groupIds"`
		GroupNames     []string `json:"groupNames"`
	}
	if err := decodePayload(payload, &p); err != nil || p.Identifier == "" {
		return fmt.Errorf("invalid client update payload")
	}
	groupIDs := p.GroupIDs
	if len(p.GroupNames) > 0 && api.GroupService != nil {
		resolved := make([]uint, 0, len(p.GroupNames))
		for _, name := range p.GroupNames {
			if id, ok := api.GroupService.GetGroupIDByName(name); ok {
				resolved = append(resolved, id)
			}
		}
		if len(resolved) > 0 {
			groupIDs = resolved
		}
	}
	return api.GroupService.ReplaceAssignments(p.Identifier, p.IdentifierType, groupIDs)
}

func (api *API) handleResolutionAddReplication(payload interface{}) error {
	var p struct{ Domain, Value, Type string }
	if err := decodePayload(payload, &p); err != nil || p.Domain == "" || p.Value == "" {
		return fmt.Errorf("invalid resolution payload")
	}
	return api.ResolutionService.CreateResolution(p.Value, p.Domain, p.Type)
}

func (api *API) handleResolutionRemoveReplication(payload interface{}) error {
	var p struct{ Domain, Value string }
	if err := decodePayload(payload, &p); err != nil || p.Domain == "" {
		return fmt.Errorf("invalid resolution remove payload")
	}
	_, err := api.ResolutionService.DeleteResolution(p.Value, p.Domain)
	return err
}

// Leases are written through their natural key, never through the Primary's autoincrement ID,
// so redelivery is idempotent and rows never collide with local ones.
func (api *API) handleDHCPLeasev6Replication(payload interface{}) error {
	var lease database.ActiveDHCPv6Lease
	if err := decodePayload(payload, &lease); err != nil || lease.DUID == "" {
		return fmt.Errorf("invalid dhcpv6 lease payload")
	}
	lease.ID = 0
	var existing database.ActiveDHCPv6Lease
	return api.DBConn.Where("duid = ? AND iaid = ?", lease.DUID, lease.IAID).Assign(lease).FirstOrCreate(&existing).Error
}

func (api *API) handleDHCPLeaseReplication(payload interface{}) error {
	var lease database.ActiveDHCPLease
	if err := decodePayload(payload, &lease); err != nil || lease.MAC == "" {
		return fmt.Errorf("invalid dhcp lease payload")
	}
	lease.ID = 0
	var existing database.ActiveDHCPLease
	return api.DBConn.Where("mac = ?", lease.MAC).Assign(lease).FirstOrCreate(&existing).Error
}

func (api *API) handleDHCPStaticAddReplication(payload interface{}) error {
	var lease database.StaticDHCPLease
	if err := decodePayload(payload, &lease); err != nil || lease.MAC == "" {
		return fmt.Errorf("invalid dhcp static payload")
	}
	lease.ID = 0
	var existing database.StaticDHCPLease
	return api.DBConn.Where("mac = ?", lease.MAC).Assign(lease).FirstOrCreate(&existing).Error
}

func (api *API) handleDHCPStaticRemoveReplication(payload interface{}) error {
	var lease database.StaticDHCPLease
	if err := decodePayload(payload, &lease); err != nil || lease.MAC == "" {
		return fmt.Errorf("invalid dhcp static remove payload")
	}
	return api.DBConn.Where("mac = ?", lease.MAC).Delete(&database.StaticDHCPLease{}).Error
}
