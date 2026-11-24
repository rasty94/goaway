package api

import (
	"context"
	"encoding/json"
	"fmt"
	"goaway/backend/audit"
	"goaway/backend/cluster"
	"goaway/backend/database"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ClusterSnapshot carries logical state that can be applied on a replica in-place without restarting.
type ClusterSnapshot struct {
	GeneratedAt  time.Time                  `json:"generatedAt"`
	Whitelist    []string                   `json:"whitelist"`
	Resolutions  []database.Resolution      `json:"resolutions"`
	StaticLeases []database.StaticDHCPLease `json:"staticLeases"`
}

type resyncReport struct {
	WhitelistAdded    int `json:"whitelistAdded"`
	ResolutionsAdded  int `json:"resolutionsAdded"`
	StaticLeasesAdded int `json:"staticLeasesAdded"`
}

func (api *API) buildClusterSnapshot() ClusterSnapshot {
	snap := ClusterSnapshot{
		GeneratedAt: time.Now().UTC(),
	}

	if api.WhitelistService != nil {
		if domains, err := api.WhitelistService.GetDomains(); err == nil {
			for d := range domains {
				snap.Whitelist = append(snap.Whitelist, d)
			}
		}
	}
	if api.ResolutionService != nil {
		if res, err := api.ResolutionService.GetResolutions(); err == nil {
			snap.Resolutions = res
		}
	}
	if api.DHCPService != nil {
		if leases, err := api.DHCPService.ListStaticLeases(); err == nil {
			snap.StaticLeases = leases
		}
	}
	return snap
}

func (api *API) applyClusterSnapshot(snap ClusterSnapshot) resyncReport {
	var rep resyncReport

	if api.WhitelistService != nil {
		for _, d := range snap.Whitelist {
			if !api.WhitelistService.IsWhitelisted(d) {
				if err := api.WhitelistService.AddDomain(d); err == nil {
					rep.WhitelistAdded++
				}
			}
		}
	}

	if api.ResolutionService != nil {
		existing := map[string]bool{}
		if rows, err := api.ResolutionService.GetResolutions(); err == nil {
			for _, r := range rows {
				existing[r.Domain+"|"+r.Value] = true
			}
		}
		for _, r := range snap.Resolutions {
			if !existing[r.Domain+"|"+r.Value] {
				if err := api.ResolutionService.CreateResolution(r.Value, r.Domain, r.Type); err == nil {
					rep.ResolutionsAdded++
				}
			}
		}
	}

	if api.DHCPService != nil {
		existingMAC := map[string]bool{}
		existingIP := map[string]bool{}
		if leases, err := api.DHCPService.ListStaticLeases(); err == nil {
			for _, l := range leases {
				existingMAC[strings.ToLower(l.MAC)] = true
				existingIP[l.IP] = true
			}
		}
		for _, l := range snap.StaticLeases {
			if !existingMAC[strings.ToLower(l.MAC)] && !existingIP[l.IP] {
				newLease := &database.StaticDHCPLease{
					MAC:      l.MAC,
					IP:       l.IP,
					Hostname: l.Hostname,
					Enabled:  l.Enabled,
				}
				if err := api.DHCPService.CreateStaticLease(newLease); err == nil {
					rep.StaticLeasesAdded++
				}
			}
		}
	}

	return rep
}

// handleClusterSnapshot exports a signed logical snapshot for hot-resyncing an out-of-sync peer.
func (api *API) handleClusterSnapshot(c *gin.Context) {
	snap := api.buildClusterSnapshot()
	payload, err := json.Marshal(snap)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encode cluster snapshot"})
		return
	}
	cluster.SignHeaders(c.Writer.Header(), api.Config.HighAvailability.Secret, c.Request.URL.Path, payload)
	c.Data(http.StatusOK, "application/json", payload)
}

func (api *API) fetchClusterSnapshot(ctx context.Context, peerAddr string) (ClusterSnapshot, error) {
	targetURL := strings.TrimRight(peerAddr, "/") + "/api/native/cluster/snapshot"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return ClusterSnapshot{}, err
	}
	cluster.SignRequest(req, api.Config.HighAvailability.Secret, nil)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ClusterSnapshot{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ClusterSnapshot{}, fmt.Errorf("peer returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return ClusterSnapshot{}, err
	}

	if err := cluster.VerifyRequest(resp.Header, api.Config.HighAvailability.Secret, "/api/native/cluster/snapshot", body, time.Now()); err != nil {
		return ClusterSnapshot{}, fmt.Errorf("invalid snapshot signature: %w", err)
	}

	var snap ClusterSnapshot
	if err := json.Unmarshal(body, &snap); err != nil {
		return ClusterSnapshot{}, err
	}
	return snap, nil
}

// triggerLogicalResync pulls a signed logical snapshot from the Primary and applies it in-place without restarting.
func (api *API) triggerLogicalResync(c *gin.Context) {
	if api.ClusterManager != nil && api.ClusterManager.IsPrimary() {
		c.JSON(http.StatusConflict, gin.H{"error": "cannot resync a primary node from a peer"})
		return
	}

	var input struct {
		Peer string `json:"peer"`
	}
	_ = c.BindJSON(&input)

	peerAddr := strings.TrimSpace(input.Peer)
	if peerAddr == "" && api.ClusterManager != nil {
		for _, n := range api.ClusterManager.GetNodes() {
			if !n.Unreachable && n.Role == cluster.RolePrimary {
				peerAddr = n.Address
				break
			}
		}
	}
	if peerAddr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no online primary peer available for resync"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	snap, err := api.fetchClusterSnapshot(ctx, peerAddr)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("failed to fetch snapshot: %v", err)})
		return
	}

	rep := api.applyClusterSnapshot(snap)
	if api.ClusterManager != nil {
		api.ClusterManager.ClearResync(peerAddr)
	}
	if api.AuditService != nil {
		api.AuditService.CreateAudit(&audit.Entry{
			Topic:     audit.TopicServer,
			Message:   fmt.Sprintf("Logical HA resync from %s completed (whitelist:+%d, resolutions:+%d, leases:+%d)", peerAddr, rep.WhitelistAdded, rep.ResolutionsAdded, rep.StaticLeasesAdded),
			CreatedAt: time.Now(),
		})
	}

	c.JSON(http.StatusOK, gin.H{"status": "synced", "peer": peerAddr, "applied": rep})
}
