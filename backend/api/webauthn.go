package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"goaway/backend/audit"
	"goaway/backend/database"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/webauthn"
)

type webAuthnUser struct {
	username    string
	credentials []webauthn.Credential
}

func (u *webAuthnUser) WebAuthnID() []byte                         { return []byte(u.username) }
func (u *webAuthnUser) WebAuthnName() string                       { return u.username }
func (u *webAuthnUser) WebAuthnDisplayName() string                { return u.username }
func (u *webAuthnUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }

type webAuthnSessionEntry struct {
	data      *webauthn.SessionData
	username  string
	name      string
	expiresAt time.Time
}

var (
	webAuthnSessionsMu sync.Mutex
	webAuthnSessions   = map[string]webAuthnSessionEntry{}
)

func (api *API) registerWebAuthnRoutes() {
	api.router.GET("/api/auth/webauthn/status", api.handleWebAuthnStatus)
	api.router.POST("/api/auth/webauthn/login/begin", api.handleWebAuthnLoginBegin)
	api.router.POST("/api/auth/webauthn/login/finish", api.handleWebAuthnLoginFinish)

	api.routes.GET("/auth/webauthn/credentials", api.handleWebAuthnListCredentials)
	api.routes.DELETE("/auth/webauthn/credentials/:id", api.handleWebAuthnDeleteCredential)
	api.routes.POST("/auth/webauthn/register/begin", api.handleWebAuthnRegisterBegin)
	api.routes.POST("/auth/webauthn/register/finish", api.handleWebAuthnRegisterFinish)
}

func (api *API) newWebAuthn() (*webauthn.WebAuthn, error) {
	if api.Config == nil {
		return nil, fmt.Errorf("config unavailable")
	}
	api.Config.RLockConfig()
	cfg := api.Config.API.WebAuthn
	api.Config.RUnlockConfig()

	if !cfg.Enabled {
		return nil, fmt.Errorf("webauthn is not enabled")
	}
	rpDisplay := strings.TrimSpace(cfg.RPDisplayName)
	if rpDisplay == "" {
		rpDisplay = "GoAway DNS"
	}
	rpID := strings.TrimSpace(cfg.RPID)
	if rpID == "" {
		rpID = "localhost"
	}
	origins := cfg.RPOrigins
	if len(origins) == 0 {
		origins = []string{"http://localhost:8080", "https://localhost:8080"}
	}

	return webauthn.New(&webauthn.Config{
		RPDisplayName: rpDisplay,
		RPID:          rpID,
		RPOrigins:     origins,
	})
}

func (api *API) loadWebAuthnUser(username string) (*webAuthnUser, error) {
	u := &webAuthnUser{username: username}
	if api.DBConn == nil {
		return u, nil
	}
	var rows []database.WebAuthnCredential
	if err := api.DBConn.Where("username = ?", username).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		var cred webauthn.Credential
		if err := json.Unmarshal([]byte(r.CredentialJSON), &cred); err == nil {
			u.credentials = append(u.credentials, cred)
		}
	}
	return u, nil
}

func storeWebAuthnSession(key string, entry webAuthnSessionEntry) {
	now := time.Now()
	webAuthnSessionsMu.Lock()
	defer webAuthnSessionsMu.Unlock()
	for k, v := range webAuthnSessions {
		if now.After(v.expiresAt) {
			delete(webAuthnSessions, k)
		}
	}
	entry.expiresAt = now.Add(5 * time.Minute)
	webAuthnSessions[key] = entry
}

func popWebAuthnSession(key string) (webAuthnSessionEntry, bool) {
	webAuthnSessionsMu.Lock()
	defer webAuthnSessionsMu.Unlock()
	entry, ok := webAuthnSessions[key]
	if !ok {
		return webAuthnSessionEntry{}, false
	}
	delete(webAuthnSessions, key)
	if time.Now().After(entry.expiresAt) {
		return webAuthnSessionEntry{}, false
	}
	return entry, true
}

func (api *API) handleWebAuthnStatus(c *gin.Context) {
	enabled := false
	if api.Config != nil {
		api.Config.RLockConfig()
		enabled = api.Config.API.WebAuthn.Enabled
		api.Config.RUnlockConfig()
	}
	c.JSON(http.StatusOK, gin.H{"enabled": enabled})
}

func (api *API) handleWebAuthnListCredentials(c *gin.Context) {
	if api.DBConn == nil {
		c.JSON(http.StatusOK, []database.WebAuthnCredential{})
		return
	}
	username := api.currentUsername(c)
	var rows []database.WebAuthnCredential
	if err := api.DBConn.Where("username = ?", username).Order("created_at DESC").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list credentials"})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (api *API) handleWebAuthnDeleteCredential(c *gin.Context) {
	if api.DBConn == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid credential id"})
		return
	}
	username := api.currentUsername(c)
	res := api.DBConn.Where("id = ? AND username = ?", id, username).Delete(&database.WebAuthnCredential{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete credential"})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "credential not found"})
		return
	}
	if api.AuditService != nil {
		api.AuditService.CreateAudit(&audit.Entry{
			Topic:     audit.TopicUser,
			Message:   fmt.Sprintf("Removed WebAuthn passkey #%d for user %s", id, username),
			CreatedAt: time.Now(),
		})
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

func (api *API) handleWebAuthnRegisterBegin(c *gin.Context) {
	wa, err := api.newWebAuthn()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	_ = c.BindJSON(&input)
	if strings.TrimSpace(input.Name) == "" {
		input.Name = "Passkey"
	}

	username := api.currentUsername(c)
	u, err := api.loadWebAuthnUser(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load user credentials"})
		return
	}

	options, session, err := wa.BeginRegistration(u)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("webauthn begin registration failed: %v", err)})
		return
	}

	sessionID, err := randomHex(16)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session id"})
		return
	}
	storeWebAuthnSession(sessionID, webAuthnSessionEntry{
		data:     session,
		username: username,
		name:     strings.TrimSpace(input.Name),
	})

	c.JSON(http.StatusOK, gin.H{"sessionId": sessionID, "options": options})
}

func (api *API) handleWebAuthnRegisterFinish(c *gin.Context) {
	wa, err := api.newWebAuthn()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	sessionID := c.Query("sessionId")
	entry, ok := popWebAuthnSession(sessionID)
	if !ok || entry.username != api.currentUsername(c) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired registration session"})
		return
	}

	u, err := api.loadWebAuthnUser(entry.username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load user"})
		return
	}

	cred, err := wa.FinishRegistration(u, *entry.data, c.Request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("webauthn registration verification failed: %v", err)})
		return
	}

	rawCred, err := json.Marshal(cred)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encode credential"})
		return
	}

	row := database.WebAuthnCredential{
		Username:       entry.username,
		Name:           entry.name,
		CredentialID:   base64.RawURLEncoding.EncodeToString(cred.ID),
		CredentialJSON: string(rawCred),
		CreatedAt:      time.Now(),
	}
	if err := api.DBConn.Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store passkey"})
		return
	}
	if api.AuditService != nil {
		api.AuditService.CreateAudit(&audit.Entry{
			Topic:     audit.TopicUser,
			Message:   fmt.Sprintf("Registered WebAuthn passkey '%s' for user %s", row.Name, entry.username),
			CreatedAt: time.Now(),
		})
	}
	c.JSON(http.StatusOK, gin.H{"id": row.ID, "name": row.Name})
}

func (api *API) handleWebAuthnLoginBegin(c *gin.Context) {
	wa, err := api.newWebAuthn()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	var input struct {
		Username string `json:"username"`
	}
	if err := c.BindJSON(&input); err != nil || strings.TrimSpace(input.Username) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username is required"})
		return
	}

	u, err := api.loadWebAuthnUser(strings.TrimSpace(input.Username))
	if err != nil || len(u.credentials) == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no passkeys registered for user"})
		return
	}

	options, session, err := wa.BeginLogin(u)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("webauthn begin login failed: %v", err)})
		return
	}

	sessionID, err := randomHex(16)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session id"})
		return
	}
	storeWebAuthnSession(sessionID, webAuthnSessionEntry{
		data:     session,
		username: u.username,
	})
	c.JSON(http.StatusOK, gin.H{"sessionId": sessionID, "options": options})
}

func (api *API) handleWebAuthnLoginFinish(c *gin.Context) {
	wa, err := api.newWebAuthn()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	sessionID := c.Query("sessionId")
	entry, ok := popWebAuthnSession(sessionID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired login session"})
		return
	}

	u, err := api.loadWebAuthnUser(entry.username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load user"})
		return
	}

	cred, err := wa.FinishLogin(u, *entry.data, c.Request)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("passkey verification failed: %v", err)})
		return
	}

	if rawCred, err := json.Marshal(cred); err == nil && api.DBConn != nil {
		credID := base64.RawURLEncoding.EncodeToString(cred.ID)
		api.DBConn.Model(&database.WebAuthnCredential{}).
			Where("credential_id = ?", credID).
			Update("credential_json", string(rawCred))
	}

	token, err := generateToken(entry.username, api.Config.API.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate session token"})
		return
	}

	role := "admin"
	if api.UserService != nil {
		if userObj, _ := api.UserService.GetUser(entry.username); userObj != nil && userObj.Role != "" {
			role = userObj.Role
		}
	}

	setAuthCookie(c.Writer, token)
	c.JSON(http.StatusOK, gin.H{"message": "Passkey login successful", "username": entry.username, "role": role})
}
