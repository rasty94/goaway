package api

import (
	"embed"
	"fmt"
	"goaway/backend/api/models"
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
            <p class="text-3xl font-black mt-1">%%d</p>
        </div>
        <div class="glass p-6 rounded-2xl stat-card border-l-4 border-l-red-500">
            <p class="text-stone-500 text-xs font-bold uppercase tracking-widest text-red-400">Blocked</p>
            <p class="text-3xl font-black mt-1 text-red-500">%%d</p>
        </div>
        <div class="glass p-6 rounded-2xl stat-card border-l-4 border-l-green-500">
            <p class="text-stone-500 text-xs font-bold uppercase tracking-widest text-green-400">Percentage</p>
            <p class="text-3xl font-black mt-1 text-green-500">%%.1f%%%%</p>
        </div>
        <div class="glass p-6 rounded-2xl stat-card border-l-4 border-l-blue-500">
            <p class="text-stone-500 text-xs font-bold uppercase tracking-widest text-blue-400">Cached</p>
            <p class="text-3xl font-black mt-1 text-blue-500">%%d</p>
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
