package api

import (
	"context"
	"encoding/json"
	"fmt"
	"goaway/backend/audit"
	"goaway/backend/settings"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"codeberg.org/miekg/dns"
	"codeberg.org/miekg/dns/dnsutil"
	"github.com/gin-gonic/gin"
	probing "github.com/prometheus-community/pro-bing"
)

type pingResult struct {
	Error      error
	Method     string
	Duration   time.Duration
	Successful bool
}

func (pr pingResult) String() string {
	if !pr.Successful {
		return fmt.Sprintf("Failed (%s)", pr.Method)
	}
	return pr.Duration.String()
}

func (api *API) registerUpstreamRoutes() {
	api.routes.POST("/upstream", api.createUpstream)
	api.routes.GET("/upstreams", api.getUpstreams)
	api.routes.PUT("/preferredUpstream", api.updatePreferredUpstream)
	api.routes.DELETE("/upstream", api.deleteUpstream)
}

func (api *API) createUpstream(c *gin.Context) {
	type UpstreamRequest struct {
		Upstream string `json:"upstream"`
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("Failed to read request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var request UpstreamRequest
	if err := json.Unmarshal(body, &request); err != nil {
		log.Error("Failed to parse JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	upstream := request.Upstream
	if upstream == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Upstream is required"})
		return
	}

	if !strings.Contains(upstream, ":") {
		upstream += ":53"
	}

	if slices.ContainsFunc(api.Config.DNS.Upstream.Servers, func(s settings.UpstreamServer) bool {
		return s.Address == upstream
	}) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Upstream already exists"})
		return
	}

	name := upstream
	if host, _, err := net.SplitHostPort(upstream); err == nil {
		name = getUpstreamName(host)
	}

	cur := api.Config.DNS.Upstream.Servers
	// copy-on-write: the resolver ranges over Servers concurrently
	next := make([]settings.UpstreamServer, 0, len(cur)+1)
	next = append(next, cur...)
	next = append(next, settings.UpstreamServer{
		Name:     name,
		Address:  upstream,
		Protocol: "udp",
		Enabled:  true,
	})
	api.Config.DNS.Upstream.Servers = next
	api.Config.Save()

	log.Info("Added %s as a new upstream", upstream)
	api.auditUpstream(fmt.Sprintf("Added a new upstream '%s'", request.Upstream))

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Added %s as a new upstream", upstream)})
}

func (api *API) auditUpstream(message string) {
	if api.DNSServer == nil || api.DNSServer.AuditService == nil {
		return
	}
	api.DNSServer.AuditService.CreateAudit(&audit.Entry{
		Topic:   audit.TopicUpstream,
		Message: message,
	})
}

func (api *API) getUpstreams(c *gin.Context) {
	var (
		servers           = api.Config.DNS.Upstream.Servers
		preferredUpstream = ""
		upstreamsToCheck  = make([]string, 0, len(servers))
		wg                sync.WaitGroup
	)

	if len(servers) > 0 {
		preferredUpstream = servers[0].Address
	}
	for _, s := range servers {
		upstreamsToCheck = append(upstreamsToCheck, s.Address)
	}

	results := make([]map[string]any, len(upstreamsToCheck))
	wg.Add(len(upstreamsToCheck))

	for i, upstream := range upstreamsToCheck {
		go func(i int, upstream string) {
			defer wg.Done()
			results[i] = getUpstreamDetails(upstream, preferredUpstream)
		}(i, upstream)
	}

	wg.Wait()

	c.JSON(http.StatusOK, gin.H{
		"upstreams":         results,
		"preferredUpstream": preferredUpstream,
	})
}

func getUpstreamDetails(upstream, preferredUpstream string) map[string]any {
	host, port, err := net.SplitHostPort(upstream)
	if err != nil {
		host = upstream
		port = "53"
		upstream = net.JoinHostPort(host, port)
	}

	entry := map[string]any{
		"upstream":  upstream,
		"preferred": upstream == preferredUpstream,
		"host":      host,
		"port":      port,
	}

	entry["resolvedIP"] = resolveHostname(host)
	entry["upstreamName"] = getUpstreamName(host)

	dnsPingResult := measureDNSPing(upstream)
	entry["dnsPing"] = dnsPingResult.String()
	entry["dnsPingSuccess"] = dnsPingResult.Successful

	icmpPingResult := measureICMPPing(host)
	entry["icmpPing"] = icmpPingResult.String()
	entry["icmpPingSuccess"] = icmpPingResult.Successful

	return entry
}

func resolveHostname(host string) string {
	if ip := net.ParseIP(host); ip != nil {
		return host
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Sprintf("Resolution failed: %v", err)
	}

	if len(ips) > 0 {
		return ips[0].IP.String()
	}

	return "No IP found"
}

func measureDNSPing(upstream string) pingResult {
	var (
		testDomains = []string{"google.com", "cloudflare.com", "quad9.net"}
		client      = &dns.Client{
			Transport: &dns.Transport{
				Dialer: &net.Dialer{
					Timeout: 3 * time.Second,
				},
			},
		}
		totalDuration time.Duration
		successCount  int
		lastError     error
	)

	for _, domain := range testDomains {
		msg := dns.NewMsg(dnsutil.Fqdn(domain), dns.TypeA)
		msg.RecursionDesired = true

		start := time.Now()
		response, _, err := client.Exchange(context.TODO(), msg, "udp", upstream)
		duration := time.Since(start)

		if err != nil {
			lastError = err
			continue
		}

		if response.Rcode != dns.RcodeSuccess {
			lastError = fmt.Errorf("DNS query failed with rcode: %d", response.Rcode)
			continue
		}

		totalDuration += duration
		successCount++
	}

	if successCount == 0 {
		return pingResult{
			Duration:   0,
			Error:      lastError,
			Method:     "dns",
			Successful: false,
		}
	}

	avgDuration := totalDuration / time.Duration(successCount)
	return pingResult{
		Duration:   avgDuration,
		Error:      nil,
		Method:     "dns",
		Successful: true,
	}
}

func measureICMPPing(host string) pingResult {
	icmpResult := tryICMPPing(host)
	if icmpResult.Successful {
		return icmpResult
	}

	tcpResult := tryTCPPing(host)
	return tcpResult
}

func tryICMPPing(host string) pingResult {
	pinger, err := probing.NewPinger(host)
	if err != nil {
		return pingResult{
			Duration:   0,
			Error:      err,
			Method:     "icmp",
			Successful: false,
		}
	}

	pinger.Count = 3
	pinger.Timeout = 3 * time.Second
	pinger.SetPrivileged(false)

	err = pinger.Run()
	if err != nil {
		return pingResult{
			Duration:   0,
			Error:      err,
			Method:     "icmp",
			Successful: false,
		}
	}

	stats := pinger.Statistics()
	if stats.PacketsRecv == 0 {
		return pingResult{
			Duration:   0,
			Error:      fmt.Errorf("no packets received"),
			Method:     "icmp",
			Successful: false,
		}
	}

	return pingResult{
		Duration:   stats.AvgRtt,
		Error:      nil,
		Method:     "icmp",
		Successful: true,
	}
}

func tryTCPPing(host string) pingResult {
	start := time.Now()

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, "53"), 2*time.Second)
	if err != nil {
		return pingResult{
			Duration:   0,
			Error:      err,
			Method:     "tcp",
			Successful: false,
		}
	}

	duration := time.Since(start)
	defer func() {
		_ = conn.Close()
	}()

	return pingResult{
		Duration:   duration,
		Error:      nil,
		Method:     "tcp",
		Successful: true,
	}
}

func getUpstreamName(host string) string {
	if ip := net.ParseIP(host); ip == nil {
		return host
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	names, err := net.DefaultResolver.LookupAddr(ctx, host)
	if err != nil {
		log.Info("Reverse lookup failed for %s: %v", host, err)
		return "unknown"
	}

	if len(names) > 0 {
		return strings.TrimSuffix(names[0], ".")
	}

	return "unknown"
}

func (api *API) updatePreferredUpstream(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("Failed to read request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var request struct {
		Upstream string `json:"upstream"`
	}

	if err := json.Unmarshal(body, &request); err != nil {
		log.Error("Failed to parse JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	if request.Upstream == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Upstream is required"})
		return
	}

	idx := slices.IndexFunc(api.Config.DNS.Upstream.Servers, func(s settings.UpstreamServer) bool {
		return s.Address == request.Upstream
	})
	if idx == -1 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Upstream not found"})
		return
	}

	if idx == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Preferred upstream already set to %s", request.Upstream)})
		return
	}

	cur := api.Config.DNS.Upstream.Servers
	// copy-on-write: the resolver ranges over Servers concurrently
	next := make([]settings.UpstreamServer, 0, len(cur))
	next = append(next, cur[idx])
	for i, s := range cur {
		if i != idx {
			next = append(next, s)
		}
	}
	api.Config.DNS.Upstream.Servers = next

	message := fmt.Sprintf("Preferred upstream set to %s", request.Upstream)
	log.Info("%s", message)

	api.Config.Save()
	api.auditUpstream(fmt.Sprintf("New preferred upstream '%s'", request.Upstream))

	c.JSON(http.StatusOK, gin.H{"message": message})
}

func (api *API) deleteUpstream(c *gin.Context) {
	upstreamToDelete := c.Query("upstream")

	if upstreamToDelete == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing 'upstream' query parameter"})
		return
	}

	if !slices.ContainsFunc(api.Config.DNS.Upstream.Servers, func(s settings.UpstreamServer) bool {
		return s.Address == upstreamToDelete
	}) {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Upstream %s not found", upstreamToDelete)})
		return
	}

	cur := api.Config.DNS.Upstream.Servers
	// copy-on-write: the resolver ranges over Servers concurrently
	next := make([]settings.UpstreamServer, 0, len(cur))
	for _, s := range cur {
		if s.Address != upstreamToDelete {
			next = append(next, s)
		}
	}
	api.Config.DNS.Upstream.Servers = next
	api.Config.Save()
	log.Info("Removed upstream: %s", upstreamToDelete)

	api.auditUpstream(fmt.Sprintf("Removed upstream '%s'", upstreamToDelete))

	c.JSON(http.StatusOK, gin.H{
		"message": "Upstream removed successfully",
	})
}
