package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"goaway/backend/audit"
	"goaway/backend/settings"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type oidcDiscovery struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
}

func (api *API) registerOIDCRoutes() {
	api.router.GET("/api/auth/oidc/status", api.handleOIDCStatus)
	api.router.GET("/api/auth/oidc/login", api.handleOIDCLogin)
	api.router.GET("/api/auth/oidc/callback", api.handleOIDCCallback)
	api.router.POST("/api/auth/oidc/logout", api.handleOIDCLogout)
}

func (api *API) oidcConfig() settings.OIDCConfig {
	if api.Config == nil {
		return settings.OIDCConfig{}
	}
	api.Config.RLockConfig()
	defer api.Config.RUnlockConfig()
	return api.Config.API.OIDC
}

func (api *API) handleOIDCStatus(c *gin.Context) {
	cfg := api.oidcConfig()
	c.JSON(http.StatusOK, gin.H{
		"enabled": cfg.Enabled && cfg.Issuer != "" && cfg.ClientID != "",
		"issuer":  cfg.Issuer,
	})
}

func fetchOIDCDiscovery(ctx context.Context, client *http.Client, issuer string) (oidcDiscovery, error) {
	wellKnown := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnown, nil)
	if err != nil {
		return oidcDiscovery{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return oidcDiscovery{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return oidcDiscovery{}, fmt.Errorf("OIDC discovery returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return oidcDiscovery{}, err
	}
	var doc oidcDiscovery
	if err := json.Unmarshal(body, &doc); err != nil {
		return oidcDiscovery{}, err
	}
	if doc.AuthorizationEndpoint == "" || doc.TokenEndpoint == "" {
		return oidcDiscovery{}, fmt.Errorf("incomplete OIDC discovery document")
	}
	return doc, nil
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (api *API) handleOIDCLogin(c *gin.Context) {
	cfg := api.oidcConfig()
	if !cfg.Enabled || cfg.Issuer == "" || cfg.ClientID == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "OIDC is not enabled"})
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	doc, err := fetchOIDCDiscovery(c.Request.Context(), client, cfg.Issuer)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("OIDC discovery failed: %v", err)})
		return
	}

	state, err := randomHex(16)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate OIDC state"})
		return
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "oidc_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{"openid", "profile", "email"}
	}

	authURL, err := url.Parse(doc.AuthorizationEndpoint)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid OIDC authorization endpoint"})
		return
	}
	q := authURL.Query()
	q.Set("response_type", "code")
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", cfg.RedirectURL)
	q.Set("scope", strings.Join(scopes, " "))
	q.Set("state", state)
	authURL.RawQuery = q.Encode()

	c.Redirect(http.StatusFound, authURL.String())
}

func (api *API) handleOIDCCallback(c *gin.Context) {
	cfg := api.oidcConfig()
	if !cfg.Enabled || cfg.Issuer == "" || cfg.ClientID == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "OIDC is not enabled"})
		return
	}

	state := c.Query("state")
	code := c.Query("code")
	cookieState, err := c.Cookie("oidc_state")
	if err != nil || state == "" || state != cookieState {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid OIDC state"})
		return
	}
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing authorization code"})
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	doc, err := fetchOIDCDiscovery(c.Request.Context(), client, cfg.Issuer)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "OIDC discovery failed"})
		return
	}

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {cfg.RedirectURL},
		"client_id":     {cfg.ClientID},
		"client_secret": {cfg.ClientSecret},
	}
	tokenReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, doc.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build token request"})
		return
	}
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	tokenResp, err := client.Do(tokenReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "OIDC token exchange failed"})
		return
	}
	defer tokenResp.Body.Close()

	if tokenResp.StatusCode != http.StatusOK {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "OIDC provider rejected authorization code"})
		return
	}

	var tokenPayload struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(io.LimitReader(tokenResp.Body, 1<<20)).Decode(&tokenPayload); err != nil || tokenPayload.AccessToken == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid OIDC token response"})
		return
	}

	claims := map[string]any{}
	if doc.UserinfoEndpoint != "" {
		uiReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, doc.UserinfoEndpoint, nil)
		if err == nil {
			uiReq.Header.Set("Authorization", "Bearer "+tokenPayload.AccessToken)
			if uiResp, err := client.Do(uiReq); err == nil {
				defer uiResp.Body.Close()
				if uiResp.StatusCode == http.StatusOK {
					_ = json.NewDecoder(io.LimitReader(uiResp.Body, 1<<20)).Decode(&claims)
				}
			}
		}
	}

	username := extractOIDCUsername(claims)
	role := mapOIDCRole(claims, cfg.RoleClaim, cfg.AdminGroups)

	if api.UserService != nil && !api.UserService.Exists(username) {
		randPass, _ := randomHex(24)
		if err := api.UserService.CreateUser(username, randPass, role); err != nil {
			log.Warning("Failed to auto-provision OIDC user %s: %v", username, err)
		}
	}

	token, err := generateToken(username, api.Config.API.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue session token"})
		return
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "oidc_state",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	setAuthCookie(c.Writer, token)

	if api.AuditService != nil {
		api.AuditService.CreateAudit(&audit.Entry{
			Topic:     audit.TopicUser,
			Message:   fmt.Sprintf("OIDC login for user %s (role: %s)", username, role),
			CreatedAt: time.Now(),
		})
	}

	c.JSON(http.StatusOK, gin.H{"message": "OIDC login successful", "username": username, "role": role})
}

func (api *API) handleOIDCLogout(c *gin.Context) {
	for _, name := range []string{"jwt", "oidc_state"} {
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}
	c.JSON(http.StatusOK, gin.H{"message": "Logged out"})
}

func extractOIDCUsername(claims map[string]any) string {
	for _, key := range []string{"preferred_username", "email", "sub"} {
		if v, ok := claims[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return "oidc-user"
}

func mapOIDCRole(claims map[string]any, roleClaim string, adminGroups []string) string {
	if len(adminGroups) == 0 {
		return "admin"
	}
	if strings.TrimSpace(roleClaim) == "" {
		roleClaim = "groups"
	}

	adminSet := make(map[string]struct{}, len(adminGroups))
	for _, g := range adminGroups {
		adminSet[strings.ToLower(strings.TrimSpace(g))] = struct{}{}
	}

	raw, exists := claims[roleClaim]
	if !exists {
		return "viewer"
	}

	switch v := raw.(type) {
	case string:
		if _, ok := adminSet[strings.ToLower(strings.TrimSpace(v))]; ok {
			return "admin"
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				if _, match := adminSet[strings.ToLower(strings.TrimSpace(s))]; match {
					return "admin"
				}
			}
		}
	}
	return "viewer"
}
