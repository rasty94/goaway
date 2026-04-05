package api

import (
	"bytes"
	"embed"
	"fmt"
	"goaway/backend/api/models"
	"goaway/backend/audit"
	"goaway/backend/cluster"
	arp "goaway/backend/dns"
	"goaway/backend/dns/server"
	"html/template"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

//go:embed templates/*
var templatesFS embed.FS

func (api *API) registerNativeRoutes() {
	api.router.GET("/native", api.serveNativeDashboard)
	api.router.GET("/api/native/view/:page", api.serveNativeView)
	api.router.GET("/api/native/stats", api.getNativeStats)
	api.router.GET("/api/native/logs", api.getNativeLogs)
	api.router.GET("/api/native/resolutions", api.getNativeResolutions)
	api.router.POST("/api/native/resolutions", api.addNativeResolution)
	api.router.DELETE("/api/native/resolutions", api.deleteNativeResolution)
	api.router.GET("/api/native/clients", api.getNativeClients)
	api.router.GET("/api/native/cache/status", api.getNativeCacheStatus)
	api.router.POST("/api/native/cache/toggle", api.toggleNativeCache)
	api.router.GET("/api/native/wildcards", api.getNativeWildcards)
	api.router.POST("/api/native/wildcards", api.addNativeWildcard)
	api.router.DELETE("/api/native/wildcards", api.deleteNativeWildcard)
	api.router.GET("/api/native/explain", api.explainNativeDNS)

	// New DHCP & Policy Routes
	api.router.GET("/api/native/dhcp/leases/v4", api.getNativeDHCPLeasesV4)
	api.routes.GET("/api/native/dhcp/leases/v6", api.getNativeDHCPLeasesV6)
	api.routes.GET("/api/native/dhcp/static", api.getNativeDHCPStatic)
	api.routes.GET("/api/native/policies/list", api.getNativePolicies)
	api.routes.POST("/api/native/dns/toggle", api.toggleDNS)
	api.routes.POST("/api/native/dhcp/toggle", api.toggleDHCP)
	api.routes.GET("/api/native/health/upstreams", api.getNativeUpstreamHealth)
	api.router.GET("/api/native/dashboard/cluster", api.getNativeCluster)
}

func (api *API) serveNativeView(c *gin.Context) {
	page := c.Param("page")
	if page == "" {
		page = "overview"
	}

	data, err := templatesFS.ReadFile("templates/" + page + ".html")
	if err != nil {
		c.String(http.StatusNotFound, "View not found: %s", page)
		return
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", data)
}

func (api *API) explainNativeDNS(c *gin.Context) {
	domain := c.Query("domain")
	client := c.Query("client")

	if domain == "" {
		c.Data(http.StatusOK, "text/html", []byte("<p class='text-stone-500 text-xs italic'>Enter a domain to explain...</p>"))
		return
	}

	if client == "" {
		client = c.ClientIP()
	}

	exp := api.DNSServer.Explain(domain, client)

	statusColor := "text-green-500"
	statusBg := "bg-green-500/10"
	statusBorder := "border-green-500/30"
	actionName := "ALLOWED"

	if exp.Blocked {
		statusColor = "text-red-500"
		statusBg = "bg-red-500/10"
		statusBorder = "border-red-500/30"
		actionName = "BLOCKED"
	}

	matchingHtml := ""
	for _, m := range exp.Matching {
		matchingHtml += fmt.Sprintf(`<span class="px-2 py-0.5 rounded bg-stone-800 text-stone-400 border border-stone-700 font-mono">%s</span> `, template.HTMLEscapeString(m))
	}

	html := fmt.Sprintf(`
        <div class="space-y-4 animate-in fade-in slide-in-from-top-4 duration-300">
            <div class="flex items-center justify-between">
                <span class="px-3 py-1 rounded-full text-[10px] font-black uppercase tracking-[0.2em] border %s %s %s">
                    %s
                </span>
                <span class="text-[10px] font-bold text-stone-600 uppercase tracking-widest">%s</span>
            </div>
            
            <div class="grid grid-cols-2 gap-4">
                <div class="space-y-1">
                    <p class="text-[10px] font-black text-stone-500 uppercase tracking-widest">Policy Engine</p>
                    <p class="text-sm font-bold text-stone-300">%s</p>
                </div>
                <div class="space-y-1">
                    <p class="text-[10px] font-black text-stone-500 uppercase tracking-widest">Decision Source</p>
                    <p class="text-sm font-bold text-stone-300">%s</p>
                </div>
            </div>

            <div class="space-y-2">
                <p class="text-[10px] font-black text-stone-500 uppercase tracking-widest">Matching Patterns</p>
                <div class="flex flex-wrap gap-2">%s</div>
            </div>
        </div>
    `, statusColor, statusBg, statusBorder, actionName, domain,
		template.HTMLEscapeString(exp.PolicyName),
		template.HTMLEscapeString(exp.Reason),
		matchingHtml)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

func (api *API) getNativeClients(c *gin.Context) {
	table := arp.GetARPTable()
	var html string
	for ip, mac := range table {
		html += fmt.Sprintf(`
            <tr class="border-b border-stone-800/50 hover:bg-stone-800/20 transition-colors">
                <td class="px-6 py-3 font-mono text-stone-300">%s</td>
                <td class="px-6 py-3 text-stone-500 font-mono text-xs">%s</td>
                <td class="px-6 py-3 text-right">
                    <button class="bg-stone-800 hover:bg-stone-700 text-stone-300 px-3 py-1 rounded text-[10px] font-black uppercase tracking-widest border border-stone-700 transition-colors"
                            hx-on:click="document.querySelector('input[name=value]').value = '%s'">
                        Use IP
                    </button>
                </td>
            </tr>
        `, ip, mac, ip)
	}
	if html == "" {
		html = "<tr><td colspan='3' class='p-10 text-center text-stone-600 uppercase text-[10px] font-black tracking-widest'>No local clients discovered yet</td></tr>"
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

func (api *API) serveNativeDashboard(c *gin.Context) {
	data, err := templatesFS.ReadFile("templates/dashboard.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Template not found")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", data)
}

func (api *API) getNativeStats(c *gin.Context) {
	allowed, blocked, cached, err := api.BlacklistService.GetRequestMetrics(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, "Error fetching metrics")
		return
	}

	total := allowed + blocked + cached
	blockedPercent := 0.0
	if total > 0 {
		blockedPercent = (float64(blocked) / float64(total)) * 100
	}

	html := fmt.Sprintf(`
        <div class="glass p-6 rounded-2xl stat-card">
            <p class="text-stone-500 text-xs font-bold uppercase tracking-widest">Total Queries</p>
            <p class="text-3xl font-black mt-1">%d</p>
        </div>
        <div class="glass p-6 rounded-2xl stat-card border-l-4 border-l-red-500">
            <p class="text-stone-500 text-xs font-bold uppercase tracking-widest text-red-400">Blocked</p>
            <p class="text-3xl font-black mt-1 text-red-500">%d</p>
        </div>
        <div class="glass p-6 rounded-2xl stat-card border-l-4 border-l-green-500">
            <p class="text-stone-500 text-xs font-bold uppercase tracking-widest text-green-400">Percentage</p>
            <p class="text-3xl font-black mt-1 text-green-500">%.1f%%</p>
        </div>
        <div class="glass p-6 rounded-2xl stat-card border-l-4 border-l-blue-500">
            <p class="text-stone-500 text-xs font-bold uppercase tracking-widest text-blue-400">Cached</p>
            <p class="text-3xl font-black mt-1 text-blue-500">%d</p>
        </div>
    `, total, blocked, blockedPercent, cached)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

func (api *API) getNativeLogs(c *gin.Context) {
	params := models.QueryParams{
		Page:      1,
		PageSize:  10,
		Direction: "DESC",
		Column:    "timestamp",
	}
	queries, _ := api.RequestService.FetchQueries(params)

	var html string
	for _, q := range queries {
		statusClass := "text-green-500"
		if q.Blocked {
			statusClass = "text-red-500"
		} else if q.Cached {
			statusClass = "text-blue-400"
		}

		clientName := "Unknown"
		if q.ClientInfo != nil {
			clientName = q.ClientInfo.Name
			if clientName == "" {
				clientName = q.ClientInfo.IP
			}
		}

		html += fmt.Sprintf(`
            <tr class="border-b border-stone-800/50 hover:bg-stone-800/20 transition-colors">
                <td class="px-6 py-4 font-mono text-stone-500 text-xs">%s</td>
                <td class="px-6 py-4 font-bold text-stone-300">%s</td>
                <td class="px-6 py-4 text-stone-400">%s</td>
                <td class="px-6 py-4 text-right">
                    <span class="px-2 py-1 rounded text-[10px] font-black uppercase border %s border-current opacity-70">
                        %s
                    </span>
                </td>
            </tr>
        `,
			q.Timestamp.Format("15:04:05"),
			template.HTMLEscapeString(q.Domain),
			template.HTMLEscapeString(clientName),
			statusClass,
			q.Status)
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

func (api *API) getNativeResolutions(c *gin.Context) {
	resolutions, _ := api.ResolutionService.GetResolutions()
	var html string
	for _, res := range resolutions {
		// Use hex of the domain to avoid special chars in ID
		rowID := fmt.Sprintf("res-%x", res.Domain)
		html += fmt.Sprintf(`
            <tr class="border-b border-stone-800/50 hover:bg-stone-800/20 transition-colors" id="%s">
                <td class="px-6 py-4 font-bold text-stone-300">%s</td>
                <td class="px-6 py-4 text-stone-400 font-mono text-xs">%s</td>
                <td class="px-6 py-4">
                    <span class="px-2 py-0.5 rounded text-[10px] font-black bg-blue-500/10 text-blue-400 border border-blue-500/30">
                        %s
                    </span>
                </td>
                <td class="px-6 py-4 text-right">
                    <button class="text-red-400 hover:text-red-300 transition-colors text-xs font-bold uppercase tracking-tighter"
                            hx-delete="/api/native/resolutions?domain=%s&value=%s"
                            hx-target="#%s"
                            hx-swap="outerHTML"
                            hx-confirm="Delete %s?">
                        Delete
                    </button>
                </td>
            </tr>
        `,
			rowID,
			template.HTMLEscapeString(res.Domain),
			template.HTMLEscapeString(res.Value),
			res.Type,
			template.HTMLEscapeString(res.Domain),
			template.HTMLEscapeString(res.Value),
			rowID,
			template.HTMLEscapeString(res.Domain))
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

func (api *API) addNativeResolution(c *gin.Context) {
	domain := c.PostForm("domain")
	value := c.PostForm("value")
	recType := c.PostForm("type")

	if domain == "" || value == "" {
		c.String(http.StatusBadRequest, "Missing fields")
		return
	}

	err := api.ResolutionService.CreateResolution(value, domain, recType)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Header("HX-Trigger", "refreshResolutions")
	c.Status(http.StatusCreated)
}

func (api *API) deleteNativeResolution(c *gin.Context) {
	domain := c.Query("domain")
	value := c.Query("value")

	_, err := api.ResolutionService.DeleteResolution(value, domain)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Status(http.StatusOK)
}

func (api *API) toggleNativeCache(c *gin.Context) {
	api.Config.DNS.CacheEnabled = !api.Config.DNS.CacheEnabled
	api.Config.Save()

	statusText := "Enabled"
	statusColor := "bg-green-500/20 text-green-400 border-green-500/30"
	if !api.Config.DNS.CacheEnabled {
		statusText = "Disabled"
		statusColor = "bg-red-500/20 text-red-400 border-red-500/30"
	}

	html := fmt.Sprintf(`
        <button hx-post="/api/native/cache/toggle" 
                hx-swap="outerHTML"
                class="px-4 py-2 rounded-lg text-xs font-bold uppercase tracking-widest border %s transition-all">
            DNS Cache: %s
        </button>
    `, statusColor, statusText)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

func (api *API) getNativeCacheStatus(c *gin.Context) {
	statusText := "Enabled"
	statusColor := "bg-green-500/20 text-green-400 border-green-500/30"
	if !api.Config.DNS.CacheEnabled {
		statusText = "Disabled"
		statusColor = "bg-red-500/20 text-red-400 border-red-500/30"
	}

	html := fmt.Sprintf(`
        <button hx-post="/api/native/cache/toggle" 
                hx-swap="outerHTML"
                class="px-4 py-2 rounded-lg text-xs font-bold uppercase tracking-widest border %s transition-all">
            DNS Cache: %s
        </button>
    `, statusColor, statusText)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

func (api *API) getNativeWildcards(c *gin.Context) {
	api.BlacklistService.PopulateWildcardCache(c.Request.Context()) // Ensure sync
	wildcards := api.BlacklistService.GetWildcards()
	var html string
	for _, w := range wildcards {
		rowID := fmt.Sprintf("wild-%x", w)
		html += fmt.Sprintf(`
            <tr class="border-b border-stone-800/50 hover:bg-stone-800/20 transition-colors" id="%s">
                <td class="px-6 py-4 font-bold text-stone-300">*.%s</td>
                <td class="px-6 py-4 text-right">
                    <button class="text-red-400 hover:text-red-300 transition-colors text-xs font-bold uppercase tracking-tighter"
                            hx-delete="/api/native/wildcards?domain=%s"
                            hx-target="#%s"
                            hx-swap="outerHTML"
                            hx-confirm="Delete wildcard for %s?">
                        Delete
                    </button>
                </td>
            </tr>
        `, rowID, template.HTMLEscapeString(w), template.HTMLEscapeString(w), rowID, template.HTMLEscapeString(w))
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

func (api *API) addNativeWildcard(c *gin.Context) {
	domain := c.PostForm("domain")
	if domain == "" {
		c.String(http.StatusBadRequest, "Missing domain")
		return
	}

	err := api.BlacklistService.AddWildcard(c.Request.Context(), domain)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Header("HX-Trigger", "refreshWildcards")
	c.Status(http.StatusCreated)
}

func (api *API) deleteNativeWildcard(c *gin.Context) {
	domain := c.Query("domain")
	err := api.BlacklistService.RemoveWildcard(c.Request.Context(), domain)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusOK)
}
func (api *API) getNativeDHCPLeasesV4(c *gin.Context) {
	leases, _ := api.DHCPService.ListActiveLeases()
	var html string
	for _, l := range leases {
		expiry := time.Until(l.ExpiresAt).Round(time.Second).String()
		if time.Now().After(l.ExpiresAt) {
			expiry = "Expired"
		}
		html += fmt.Sprintf(`
            <tr class="border-b border-stone-800/50 hover:bg-stone-800/20 transition-colors">
                <td class="px-10 py-3 font-mono text-stone-300">%s</td>
                <td class="px-10 py-3 text-stone-500 font-mono text-xs">%s</td>
                <td class="px-10 py-3 text-stone-400 font-bold text-[10px] uppercase">%s</td>
            </tr>
        `, l.IP, l.MAC, expiry)
	}
	if html == "" {
		html = "<tr><td colspan='3' class='p-10 text-center text-stone-700 font-bold text-xs uppercase'>No active IPv4 leases</td></tr>"
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

func (api *API) getNativeDHCPLeasesV6(c *gin.Context) {
	leases, _ := api.DHCPService.ListActivev6Leases()
	var html string
	for _, l := range leases {
		expiry := time.Until(l.ExpiresAt).Round(time.Second).String()
		if time.Now().After(l.ExpiresAt) {
			expiry = "Expired"
		}
		// DUID can be long, truncate for UI
		duidShort := l.DUID
		if len(duidShort) > 20 {
			duidShort = duidShort[:20] + "..."
		}
		html += fmt.Sprintf(`
            <tr class="border-b border-stone-800/50 hover:bg-stone-800/20 transition-colors">
                <td class="px-10 py-3 font-mono text-blue-300 text-xs">%s</td>
                <td class="px-10 py-3 text-stone-500 font-mono text-[10px]">%s</td>
                <td class="px-10 py-3 text-stone-400 font-bold text-[10px] uppercase">%s</td>
            </tr>
        `, l.IP, duidShort, expiry)
	}
	if html == "" {
		html = "<tr><td colspan='3' class='p-10 text-center text-stone-700 font-bold text-xs uppercase'>No active IPv6 leases</td></tr>"
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

func (api *API) getNativeDHCPStatic(c *gin.Context) {
	v4, _ := api.DHCPService.ListStaticLeases()
	v6, _ := api.DHCPService.ListStaticv6Leases()
	var html string
	
	for _, l := range v4 {
		html += api.renderStaticLeaseRow(l.Hostname, l.IP, l.MAC, "IPv4", l.Enabled)
	}
	for _, l := range v6 {
		html += api.renderStaticLeaseRow(l.Hostname, l.IP, l.DUID, "IPv6", l.Enabled)
	}
	
	if html == "" {
		html = "<tr><td colspan='5' class='p-10 text-center text-stone-700 font-bold text-xs'>No static reservations configured.</td></tr>"
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

func (api *API) renderStaticLeaseRow(hostname, ip, ident, proto string, enabled bool) string {
	statusText := "Active"
	statusClass := "text-green-500 bg-green-500/10 border-green-500/30"
	if !enabled {
		statusText = "Disabled"
		statusClass = "text-stone-500 bg-stone-500/10 border-stone-500/30"
	}
	
	return fmt.Sprintf(`
        <tr class="border-b border-stone-800/50 hover:bg-stone-800/20 transition-colors">
            <td class="px-10 py-4 font-bold text-stone-300">/%s</td>
            <td class="px-10 py-4 font-mono text-stone-400 text-xs">%s</td>
            <td class="px-10 py-4 font-mono text-stone-500 text-[10px]">%s</td>
            <td class="px-10 py-4"><span class="px-2 py-0.5 rounded text-[10px] font-black bg-stone-800 text-stone-500 border border-stone-700">%s</span></td>
            <td class="px-10 py-4 text-right">
                <span class="px-2 py-1 rounded text-[10px] font-black uppercase border %s">%s</span>
            </td>
        </tr>
    `, hostname, ip, ident, proto, statusClass, statusText)
}

func (api *API) getNativePolicies(c *gin.Context) {
	policies, _ := api.PolicyService.GetPolicies()
	var html string
	for _, p := range policies {
		scope := "Custom"
		// In a real scenario, we would check PolicyAssignments for the scope
		
		ssClass := "text-stone-600 bg-stone-900 border-stone-800"
		if p.SafeSearch {
			ssClass = "text-blue-400 bg-blue-500/10 border-blue-500/30"
		}
		
		html += fmt.Sprintf(`
            <tr class="border-b border-stone-800/50 hover:bg-stone-800/20 transition-colors">
                <td class="px-10 py-4 font-bold text-stone-300">%s</td>
                <td class="px-10 py-4"><span class="px-3 py-1 rounded-full text-[10px] font-black uppercase bg-stone-800 text-stone-400 border border-stone-700">%s</span></td>
                <td class="px-10 py-4"><span class="px-3 py-1 rounded-full text-[10px] font-black uppercase border %s">%t</span></td>
                <td class="px-10 py-4 text-stone-500 text-xs font-medium">All Clients</td>
                <td class="px-10 py-4 text-right">
                    <button class="text-stone-500 hover:text-white transition-colors text-xs font-bold uppercase tracking-widest">Edit</button>
                </td>
            </tr>
        `, p.Name, scope, ssClass, p.SafeSearch)
	}
	if html == "" {
		html = "<tr><td colspan='5' class='p-10 text-center text-stone-700 font-bold text-xs uppercase'>No custom policies configured</td></tr>"
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}
func (api *API) toggleDNS(c *gin.Context) {
	api.DNSServer.IsPaused = !api.DNSServer.IsPaused
	status := "Running"
	if api.DNSServer.IsPaused {
		status = "Paused"
	}
	api.AuditService.CreateAudit(&audit.Entry{
		Topic:   audit.TopicDNS,
		Message: fmt.Sprintf("DNS Service %s", status),
	})
	c.JSON(http.StatusOK, gin.H{"status": status})
}

func (api *API) toggleDHCP(c *gin.Context) {
	if api.DHCPService.IsRunning() {
		api.DHCPService.Stop()
	} else {
		if err := api.DHCPService.Start(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	status := "Stopped"
	if api.DHCPService.IsRunning() {
		status = "Running"
	}
	api.AuditService.CreateAudit(&audit.Entry{
		Topic:   audit.TopicDHCP,
		Message: fmt.Sprintf("DHCP Service %s", status),
	})
	c.JSON(http.StatusOK, gin.H{"status": status})
}

func (api *API) getNativeUpstreamHealth(c *gin.Context) {
	var html string
	api.DNSServer.UpstreamHealth.Range(func(key, value interface{}) bool {
		h := value.(*server.UpstreamHealth)
		statusColor := "text-emerald-500"
		if h.Status == "Slow" {
			statusColor = "text-yellow-500"
		} else if h.Status == "Unreachable" {
			statusColor = "text-red-500"
		}
		
		html += fmt.Sprintf(`
            <div class="flex items-center justify-between p-4 bg-stone-900/10 rounded-2xl border border-stone-800/50">
                <div class="flex items-center gap-4">
                    <div class="w-2 h-2 rounded-full %s animate-pulse bg-current shadow-[0_0_10px_currentColor]"></div>
                    <div class="min-w-0">
                        <p class="text-[11px] font-bold text-stone-300 truncate max-w-[150px]">%s</p>
                        <p class="text-[9px] font-black text-stone-600 uppercase tracking-widest">%s</p>
                    </div>
                </div>
                <div class="text-right">
                    <p class="text-xs font-mono font-bold text-stone-400">%dms</p>
                    <p class="text-[9px] font-black text-stone-600 uppercase tracking-tighter">Latency</p>
                </div>
            </div>
        `, statusColor, h.Server, h.Status, h.Latency.Milliseconds())
		return true
	})
	
	if html == "" {
		html = "<div class='p-10 text-center'><p class='text-[10px] font-black text-stone-700 uppercase tracking-widest'>No analysis data yet</p></div>"
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

func (api *API) getNativeCluster(c *gin.Context) {
	if api.ClusterManager == nil {
		c.String(http.StatusServiceUnavailable, "Cluster Manager not initialized")
		return
	}

	nodes := api.ClusterManager.GetNodes()
	count := 0
	for _, n := range nodes {
		if !n.Unreachable {
			count++
		}
	}
	
	activeNodes := count + 1

	data := struct {
		SelfRole    string
		ActiveNodes int
		ClusterID   string
		Nodes       []*cluster.ClusterNode
	}{
		SelfRole:    api.Config.HighAvailability.Mode,
		ActiveNodes: activeNodes,
		ClusterID:   api.Config.HighAvailability.ClusterID,
		Nodes:       nodes,
	}

	tmpl, err := template.ParseFS(templatesFS, "templates/cluster.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		c.String(http.StatusInternalServerError, "Execution error: %v", err)
		return
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}
