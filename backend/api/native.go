package api

import (
	"embed"
	"fmt"
	"goaway/backend/api/models"
	arp "goaway/backend/dns"
	"html/template"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed templates/*
var templatesFS embed.FS

func (api *API) registerNativeRoutes() {
	api.router.GET("/native", api.serveNativeDashboard)
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
