package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	"goaway/backend/audit"
	"goaway/backend/database"
	"goaway/backend/pihole"
	"goaway/backend/settings"

	"github.com/gin-gonic/gin"
)

func (api *API) registerPiholeImportRoutes() {
	api.routes.POST("/import/pihole/preview", api.piholeImportPreview)
	api.routes.POST("/import/pihole/apply", api.piholeImportApply)
}

// piholeImportPreview parses a Pi-hole v6 Teleporter archive and returns the plan with conflicts marked
//
//	@Summary Pi-hole import preview
//	@Description Parse a Pi-hole v6 Teleporter zip and return what would be imported, flagging entries that already exist
//	@Tags import
//	@Accept  multipart/form-data
//	@Param   backup  formData  file   true  "Pi-hole Teleporter zip"
//	@Success 200
//	@Router /import/pihole/preview [post]
func (api *API) piholeImportPreview(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 512<<20)
	file, header, err := c.Request.FormFile("backup")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "backup file is required"})
		return
	}
	defer file.Close()

	plan, err := pihole.Parse(file, header.Size)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	api.markExisting(plan)
	c.JSON(http.StatusOK, plan)
}

func (api *API) markExisting(plan *pihole.Plan) {
	for i := range plan.Adlists {
		plan.Adlists[i].Exists = api.BlacklistService.URLExists(plan.Adlists[i].URL)
	}
	wildcards := map[string]bool{}
	for _, w := range api.BlacklistService.GetWildcards() {
		wildcards[w] = true
	}
	for i := range plan.Blacklist {
		if pattern := plan.Blacklist[i].Pattern; strings.HasPrefix(pattern, "*.") {
			plan.Blacklist[i].Exists = wildcards[pattern]
		} else {
			plan.Blacklist[i].Exists = api.BlacklistService.IsBlacklisted(pattern)
		}
	}
	for i := range plan.Whitelist {
		plan.Whitelist[i].Exists = api.WhitelistService.IsWhitelisted(plan.Whitelist[i].Pattern)
	}

	groupIDs := api.groupIDsByName()
	for i := range plan.Groups {
		_, plan.Groups[i].Exists = groupIDs[plan.Groups[i].Name]
	}
	blocked, allowed := api.groupDomainSets()
	for i := range plan.GroupDomains {
		gd := &plan.GroupDomains[i]
		if id, ok := groupIDs[gd.Group]; ok {
			key := fmt.Sprintf("%d|%s", id, gd.Pattern)
			if gd.Blocked {
				gd.Exists = blocked[key]
			} else {
				gd.Exists = allowed[key]
			}
		}
	}
	for i := range plan.Clients {
		ids, _ := api.GroupService.GetAssignmentsByIdentifier(plan.Clients[i].Identifier, plan.Clients[i].Type)
		plan.Clients[i].Exists = len(ids) > 0
	}
	resolutions := map[string]bool{}
	if rows, err := api.ResolutionService.GetResolutions(); err == nil {
		for _, r := range rows {
			resolutions[r.Domain] = true
		}
	}
	for i := range plan.Resolutions {
		plan.Resolutions[i].Exists = resolutions[plan.Resolutions[i].Domain]
	}
	for i := range plan.Upstreams {
		for _, s := range api.Config.DNS.Upstream.Servers {
			if s.Address == plan.Upstreams[i].Address {
				plan.Upstreams[i].Exists = true
			}
		}
	}
	leases, _ := api.DHCPService.ListStaticLeases()
	for i := range plan.StaticLeases {
		for _, l := range leases {
			if l.MAC == plan.StaticLeases[i].MAC || l.IP == plan.StaticLeases[i].IP {
				plan.StaticLeases[i].Exists = true
			}
		}
	}
}

func (api *API) groupIDsByName() map[string]uint {
	ids := map[string]uint{}
	groups, _ := api.GroupService.GetGroups()
	for _, g := range groups {
		ids[g.Name] = g.ID
	}
	return ids
}

func (api *API) groupDomainSets() (blocked, allowed map[string]bool) {
	blocked, allowed = map[string]bool{}, map[string]bool{}
	for _, d := range api.GroupService.ListBlockedDomains() {
		blocked[fmt.Sprintf("%d|%s", d.GroupID, d.Domain)] = true
	}
	for _, d := range api.GroupService.ListAllowedDomains() {
		allowed[fmt.Sprintf("%d|%s", d.GroupID, d.Domain)] = true
	}
	return blocked, allowed
}

type piholeImportRequest struct {
	Plan     pihole.Plan     `json:"plan"`
	Sections map[string]bool `json:"sections"`
}

type piholeImportReport struct {
	Created map[string]int `json:"created"`
	Skipped map[string]int `json:"skipped"`
	Errors  []string       `json:"errors"`
}

func (r *piholeImportReport) fail(section string, err error) {
	r.Errors = append(r.Errors, section+": "+err.Error())
}

// piholeImportApply applies the reviewed plan; entries marked as existing are skipped
//
//	@Summary Pi-hole import apply
//	@Description Apply a previously previewed Pi-hole import plan, section by section
//	@Tags import
//	@Accept  application/json
//	@Success 200
//	@Router /import/pihole/apply [post]
func (api *API) piholeImportApply(c *gin.Context) {
	var req piholeImportRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid import plan"})
		return
	}
	plan := &req.Plan
	api.markExisting(plan) // never trust the client's conflict flags
	on := func(section string) bool { return req.Sections[section] }

	ctx := context.Background()
	report := piholeImportReport{Created: map[string]int{}, Skipped: map[string]int{}, Errors: []string{}}
	count := func(section string, exists bool, err error) {
		switch {
		case exists:
			report.Skipped[section]++
		case err != nil:
			report.fail(section, err)
		default:
			report.Created[section]++
		}
	}

	if on("adlists") {
		for _, a := range plan.Adlists {
			var err error
			if !a.Exists {
				if err = api.BlacklistService.AddSource(ctx, a.Name, a.URL); err == nil && !a.Enabled {
					err = api.BlacklistService.ToggleBlocklistStatus(ctx, a.Name)
				}
			}
			count("adlists", a.Exists, err)
		}
	}
	if on("blacklist") {
		var patterns []string
		seen := map[string]bool{}
		for _, d := range plan.Blacklist {
			if d.Exists {
				count("blacklist", true, nil)
				continue
			}
			if !seen[d.Pattern] {
				seen[d.Pattern] = true
				patterns = append(patterns, d.Pattern)
			}
		}
		if len(patterns) > 0 {
			if err := api.BlacklistService.AddCustomDomains(ctx, patterns); err == nil {
				report.Created["blacklist"] += len(patterns)
			} else {
				// Batch failed (e.g. one entry already exists): fall back so the report shows which entries failed.
				for _, d := range plan.Blacklist {
					if d.Exists {
						continue
					}
					var err error
					if strings.HasPrefix(d.Pattern, "*.") {
						err = api.BlacklistService.AddWildcard(ctx, d.Pattern)
					} else {
						err = api.BlacklistService.AddBlacklistedDomain(ctx, d.Pattern)
					}
					count("blacklist", false, err)
				}
			}
		}
	}
	if on("whitelist") {
		for _, d := range plan.Whitelist {
			var err error
			if !d.Exists {
				err = api.WhitelistService.AddDomain(d.Pattern)
			}
			count("whitelist", d.Exists, err)
		}
	}
	if on("groups") {
		for _, g := range plan.Groups {
			var err error
			if !g.Exists {
				_, err = api.GroupService.CreateGroup(g.Name, g.Description, true)
			}
			count("groups", g.Exists, err)
		}
	}
	groupIDs := api.groupIDsByName()
	if on("groupDomains") {
		blockedByGroup := map[string][]string{}
		allowedByGroup := map[string][]string{}
		for _, gd := range plan.GroupDomains {
			if gd.Exists {
				count("groupDomains", true, nil)
				continue
			}
			if _, ok := groupIDs[gd.Group]; !ok {
				count("groupDomains", false, fmt.Errorf("group %q does not exist; import groups first", gd.Group))
				continue
			}
			if gd.Blocked {
				blockedByGroup[gd.Group] = append(blockedByGroup[gd.Group], gd.Pattern)
			} else {
				allowedByGroup[gd.Group] = append(allowedByGroup[gd.Group], gd.Pattern)
			}
		}
		for name, patterns := range blockedByGroup {
			n, err := api.GroupService.AddBlockedDomains(groupIDs[name], patterns)
			report.Created["groupDomains"] += n
			if err != nil {
				report.fail("groupDomains", fmt.Errorf("group %q: %w", name, err))
			}
		}
		for name, patterns := range allowedByGroup {
			n, err := api.GroupService.AddAllowedDomains(groupIDs[name], patterns)
			report.Created["groupDomains"] += n
			if err != nil {
				report.fail("groupDomains", fmt.Errorf("group %q: %w", name, err))
			}
		}
	}
	if on("clients") {
		for _, cl := range plan.Clients {
			var err error
			if !cl.Exists {
				ids := make([]uint, 0, len(cl.Groups))
				for _, name := range cl.Groups {
					if id, ok := groupIDs[name]; ok {
						ids = append(ids, id)
					}
				}
				if len(ids) == 0 {
					err = fmt.Errorf("client %s: none of its groups exist; import groups first", cl.Identifier)
				} else {
					err = api.GroupService.ReplaceAssignments(cl.Identifier, cl.Type, ids)
				}
			}
			count("clients", cl.Exists, err)
		}
	}
	if on("resolutions") {
		for _, r := range plan.Resolutions {
			var err error
			if !r.Exists {
				err = api.ResolutionService.CreateResolution(r.Value, r.Domain, r.Type)
			}
			count("resolutions", r.Exists, err)
		}
	}
	if on("upstreams") {
		changed := false
		for _, u := range plan.Upstreams {
			var err error
			if !u.Exists {
				if _, _, err = net.SplitHostPort(u.Address); err == nil {
					api.Config.DNS.Upstream.Servers = append(api.Config.DNS.Upstream.Servers, settings.UpstreamServer{
						Name: "Pi-hole " + u.Address, Address: u.Address, Protocol: "udp", Enabled: true,
					})
					changed = true
				}
			}
			count("upstreams", u.Exists, err)
		}
		if changed {
			api.Config.Save()
		}
	}
	if on("staticLeases") {
		for _, l := range plan.StaticLeases {
			var err error
			if !l.Exists {
				err = api.DHCPService.CreateStaticLease(&database.StaticDHCPLease{MAC: l.MAC, IP: l.IP, Hostname: l.Hostname, Enabled: true})
			}
			count("staticLeases", l.Exists, err)
		}
	}

	total := 0
	for _, n := range report.Created {
		total += n
	}
	if api.AuditService != nil {
		api.AuditService.CreateAudit(&audit.Entry{
			Topic:   audit.TopicSettings,
			Message: fmt.Sprintf("Pi-hole import applied: %d entries created, %d errors", total, len(report.Errors)),
		})
	}
	c.JSON(http.StatusOK, report)
}
