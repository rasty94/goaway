package api

import (
	"net/http"
	"strconv"

	"goaway/backend/database"

	"github.com/gin-gonic/gin"
)

func (api *API) registerDHCPRoutes() {
	api.routes.GET("/dhcp/status", api.getDHCPStatus)
	api.routes.POST("/dhcp/start", api.startDHCP)
	api.routes.POST("/dhcp/stop", api.stopDHCP)

	api.routes.GET("/dhcp/activeLeases", api.listActiveDHCPLeases)
	api.routes.GET("/dhcp/leases", api.listStaticDHCPLeases)
	api.routes.POST("/dhcp/leases", api.createStaticDHCPLease)
	api.routes.PUT("/dhcp/leases/:id", api.updateStaticDHCPLease)
	api.routes.DELETE("/dhcp/leases/:id", api.deleteStaticDHCPLease)
}

func (api *API) getDHCPStatus(c *gin.Context) {
	if api.DHCPService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "dhcp service unavailable"})
		return
	}

	leases, err := api.DHCPService.ListStaticLeases()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled":     api.Config.DHCP.Enabled,
		"running":     api.DHCPService.IsRunning(),
		"ipv4Enabled": api.Config.DHCP.IPv4Enabled,
		"ipv6Enabled": api.Config.DHCP.IPv6Enabled,
		"leaseCount":  len(leases),
	})
}

func (api *API) startDHCP(c *gin.Context) {
	if api.DHCPService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "dhcp service unavailable"})
		return
	}

	api.Config.DHCP.Enabled = true
	api.Config.Save()

	if err := api.DHCPService.Restart(); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func (api *API) stopDHCP(c *gin.Context) {
	if api.DHCPService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "dhcp service unavailable"})
		return
	}

	api.Config.DHCP.Enabled = false
	api.Config.Save()
	api.DHCPService.Stop()
	c.Status(http.StatusOK)
}

func (api *API) listStaticDHCPLeases(c *gin.Context) {
	if api.DHCPService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "dhcp service unavailable"})
		return
	}

	leases, err := api.DHCPService.ListStaticLeases()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, leases)
}

func (api *API) createStaticDHCPLease(c *gin.Context) {
	if api.DHCPService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "dhcp service unavailable"})
		return
	}

	var lease database.StaticDHCPLease
	if err := c.BindJSON(&lease); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lease payload"})
		return
	}

	if err := api.DHCPService.CreateStaticLease(&lease); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, lease)
}

func (api *API) updateStaticDHCPLease(c *gin.Context) {
	if api.DHCPService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "dhcp service unavailable"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lease id"})
		return
	}

	var lease database.StaticDHCPLease
	if err := c.BindJSON(&lease); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lease payload"})
		return
	}

	if err := api.DHCPService.UpdateStaticLease(uint(id), &lease); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func (api *API) deleteStaticDHCPLease(c *gin.Context) {
	if api.DHCPService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "dhcp service unavailable"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lease id"})
		return
	}

	if err := api.DHCPService.DeleteStaticLease(uint(id)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func (api *API) listActiveDHCPLeases(c *gin.Context) {
	if api.DHCPService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "dhcp service unavailable"})
		return
	}

	leases, err := api.DHCPService.ListActiveLeases()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, leases)
}

