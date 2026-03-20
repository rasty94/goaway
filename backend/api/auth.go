package api

import (
	"context"
	"encoding/json"
	"fmt"
	"goaway/backend/alert"
	"goaway/backend/audit"
	"goaway/backend/user"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (api *API) registerAuthRoutes() {
	api.router.POST("/api/login", api.handleLogin)
	api.router.GET("/api/authentication", api.getAuthentication)
	api.routes.PUT("/password", api.updatePassword)

	api.routes.POST("/users", api.requireAdmin(), api.createUser)
	api.routes.GET("/users", api.requireAdmin(), api.getUsers)
	api.routes.DELETE("/users/:username", api.requireAdmin(), api.deleteUser)

	api.routes.POST("/apiKey", api.createAPIKey)
	api.routes.GET("/apiKey", api.getAPIKeys)
	api.routes.GET("/deleteApiKey", api.deleteAPIKey)
}

func (api *API) handleLogin(c *gin.Context) {
	allowed, timeUntilReset := api.RateLimiter.CheckLimit(c.ClientIP())
	if !allowed {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":             "Too many login attempts. Please try again later.",
			"retryAfterSeconds": timeUntilReset,
		})
		return
	}

	var loginUser user.User
	if err := c.BindJSON(&loginUser); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	if err := api.UserService.ValidateCredentials(loginUser); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	if api.UserService.Authenticate(loginUser.Username, loginUser.Password) {
		token, err := generateToken(loginUser.Username, api.Config.API.JWTSecret)
		if err != nil {
			log.Info("Token generation failed for user %s: %v", loginUser.Username, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Authentication service temporarily unavailable",
			})
			return
		}

		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")

		userObj, _ := api.UserService.GetUser(loginUser.Username)
		role := "admin"
		if userObj != nil && userObj.Role != "" {
			role = userObj.Role
		}

		setAuthCookie(c.Writer, token)
		c.JSON(http.StatusOK, gin.H{"message": "Login successful", "role": role})
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid username or password",
		})
	}
}

func (api *API) getAuthentication(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"enabled": api.Authentication})
}

func (api *API) updatePassword(c *gin.Context) {
	type passwordChange struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}

	var newCredentials passwordChange
	if err := c.BindJSON(&newCredentials); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	loggedInUser := c.GetString("username")

	if !api.UserService.Authenticate(loggedInUser, newCredentials.CurrentPassword) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Current password is not valid"})
		return
	}

	if err := api.UserService.UpdatePassword(loggedInUser, newCredentials.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unable to update password"})
		return
	}

	logMsg := fmt.Sprintf("Password changed for user '%s'", loggedInUser)
	api.DNSServer.AuditService.CreateAudit(&audit.Entry{
		Topic:   audit.TopicUser,
		Message: logMsg,
	})
	go func() {
		_ = api.DNSServer.AlertService.SendToAll(context.Background(), alert.Message{
			Title:    "System",
			Content:  logMsg,
			Severity: SeverityWarning,
		})
	}()

	log.Warning("%s", logMsg)
	c.Status(http.StatusOK)
}

func (api *API) createAPIKey(c *gin.Context) {
	type NewAPIKeyName struct {
		Name   string   `json:"name"`
		Scopes []string `json:"scopes"`
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("Failed to read request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var request NewAPIKeyName
	if err := json.Unmarshal(body, &request); err != nil {
		log.Error("Failed to parse JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	// Default scope if none provided
	if len(request.Scopes) == 0 {
		request.Scopes = []string{"read", "admin"}
	}

	apiKey, err := api.KeyService.CreateKey(request.Name, request.Scopes)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	go func() {
		_ = api.DNSServer.AlertService.SendToAll(context.Background(), alert.Message{
			Title:    "System",
			Content:  fmt.Sprintf("New API key created with the name '%s' and scopes '%v'", request.Name, request.Scopes),
			Severity: SeverityWarning,
		})
	}()

	c.JSON(http.StatusOK, apiKey)
}

func (api *API) getAPIKeys(c *gin.Context) {
	apiKeys, err := api.KeyService.GetAllKeys()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, apiKeys)
}

func (api *API) deleteAPIKey(c *gin.Context) {
	keyName := c.Query("name")

	err := api.KeyService.DeleteKey(keyName)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted api key!"})
}

func (api *API) requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		username := c.GetString("username")
		userObj, err := api.UserService.GetUser(username)
		if err != nil || userObj == nil || userObj.Role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin access required"})
			return
		}
		c.Next()
	}
}

func (api *API) roleMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "GET" || c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		// Allow password update for self even if viewer
		if c.Request.URL.Path == "/api/password" && c.Request.Method == "PUT" {
			c.Next()
			return
		}

		if apiKey := c.GetHeader("api-key"); apiKey != "" {
			if api.KeyService.VerifyKeyScope(apiKey, "admin") {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin scope required for this action"})
			return
		}

		username := c.GetString("username")
		userObj, err := api.UserService.GetUser(username)
		if err != nil || userObj == nil || userObj.Role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin role required for this action"})
			return
		}
		c.Next()
	}
}

func (api *API) createUser(c *gin.Context) {
	var newUser user.User
	if err := c.BindJSON(&newUser); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	if err := api.UserService.ValidateCredentials(newUser); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := api.UserService.CreateUser(newUser.Username, newUser.Password, newUser.Role); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unable to create user"})
		return
	}

	api.DNSServer.AuditService.CreateAudit(&audit.Entry{
		Topic:   audit.TopicUser,
		Message: fmt.Sprintf("Created new user '%s' with role '%s'", newUser.Username, newUser.Role),
	})

	c.Status(http.StatusCreated)
}

func (api *API) getUsers(c *gin.Context) {
	users, err := api.UserService.GetAllUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to fetch users"})
		return
	}
	c.JSON(http.StatusOK, users)
}

func (api *API) deleteUser(c *gin.Context) {
	username := c.Param("username")
	if err := api.UserService.DeleteUser(username); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	api.DNSServer.AuditService.CreateAudit(&audit.Entry{
		Topic:   audit.TopicUser,
		Message: fmt.Sprintf("Deleted user '%s'", username),
	})

	c.Status(http.StatusOK)
}
