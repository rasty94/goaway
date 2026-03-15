package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func (api *API) registerGroupRoutes() {
	api.routes.GET("/groups", api.getGroups)
	api.routes.POST("/groups", api.createGroup)
	api.routes.DELETE("/groups/:id", api.deleteGroup)
	api.routes.PUT("/groups/:id/global/:enabled", api.updateGroupGlobalPolicy)

	api.routes.POST("/groups/:id/blocked", api.addGroupBlockedDomain)
	api.routes.DELETE("/groups/:id/blocked", api.removeGroupBlockedDomain)
	api.routes.POST("/groups/:id/allowed", api.addGroupAllowedDomain)
	api.routes.DELETE("/groups/:id/allowed", api.removeGroupAllowedDomain)

	api.routes.PUT("/groups/assignments", api.replaceGroupAssignments)
	api.routes.GET("/groups/assignments", api.getGroupAssignments)
	api.routes.GET("/groups/effective", api.getEffectiveGroupPolicy)
}

func (api *API) getGroups(c *gin.Context) {
	groups, err := api.GroupService.GetGroups()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, groups)
}

func (api *API) createGroup(c *gin.Context) {
	var payload struct {
		Name              string `json:"name"`
		Description       string `json:"description"`
		UseGlobalPolicies bool   `json:"useGlobalPolicies"`
	}

	if err := c.BindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group payload"})
		return
	}

	group, err := api.GroupService.CreateGroup(payload.Name, payload.Description, payload.UseGlobalPolicies)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, group)
}

func (api *API) deleteGroup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}

	if err := api.GroupService.DeleteGroup(uint(id)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func (api *API) updateGroupGlobalPolicy(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}

	enabled := strings.EqualFold(c.Param("enabled"), "true")
	if c.Param("enabled") != "true" && c.Param("enabled") != "false" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "enabled must be true or false"})
		return
	}

	if err := api.GroupService.SetGroupUseGlobalPolicies(uint(id), enabled); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func (api *API) addGroupBlockedDomain(c *gin.Context) {
	groupID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}

	var payload struct {
		Domain string `json:"domain"`
	}
	if err := c.BindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if err := api.GroupService.AddBlockedDomain(uint(groupID), payload.Domain); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func (api *API) removeGroupBlockedDomain(c *gin.Context) {
	groupID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}

	domain := c.Query("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain query param is required"})
		return
	}

	if err := api.GroupService.RemoveBlockedDomain(uint(groupID), domain); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func (api *API) addGroupAllowedDomain(c *gin.Context) {
	groupID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}

	var payload struct {
		Domain string `json:"domain"`
	}
	if err := c.BindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if err := api.GroupService.AddAllowedDomain(uint(groupID), payload.Domain); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func (api *API) removeGroupAllowedDomain(c *gin.Context) {
	groupID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}

	domain := c.Query("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain query param is required"})
		return
	}

	if err := api.GroupService.RemoveAllowedDomain(uint(groupID), domain); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func (api *API) replaceGroupAssignments(c *gin.Context) {
	var payload struct {
		Identifier     string `json:"identifier"`
		IdentifierType string `json:"identifierType"`
		GroupIDs       []uint `json:"groupIDs"`
	}

	if err := c.BindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid assignment payload"})
		return
	}

	if err := api.GroupService.ReplaceAssignments(payload.Identifier, payload.IdentifierType, payload.GroupIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func (api *API) getGroupAssignments(c *gin.Context) {
	identifier := c.Query("identifier")
	identifierType := c.DefaultQuery("identifierType", "ip")

	if identifier == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "identifier query param is required"})
		return
	}

	groupIDs, err := api.GroupService.GetAssignmentsByIdentifier(identifier, identifierType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"groupIDs": groupIDs})
}

func (api *API) getEffectiveGroupPolicy(c *gin.Context) {
	ip := c.Query("ip")
	mac := c.Query("mac")
	policy := api.GroupService.GetEffectivePolicy(ip, mac)
	c.JSON(http.StatusOK, policy)
}
