package api

import (
	"encoding/base64"
	"encoding/json"
	"goaway/backend/database"
	"goaway/backend/settings"
	"goaway/backend/user"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestOIDCLoginCallbackAndRoleMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var providerURL string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(oidcDiscovery{
				AuthorizationEndpoint: providerURL + "/authorize",
				TokenEndpoint:         providerURL + "/token",
				UserinfoEndpoint:      providerURL + "/userinfo",
			})
		case "/token":
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "tok-123"})
		case "/userinfo":
			assert.Equal(t, "Bearer tok-123", r.Header.Get("Authorization"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"preferred_username": "carol",
				"groups":             []string{"dns-admins"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	providerURL = provider.URL
	t.Cleanup(provider.Close)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&database.User{}))

	jwtSecret := base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	cfg := &settings.Config{
		API: settings.APIConfig{
			JWTSecret: jwtSecret,
			OIDC: settings.OIDCConfig{
				Enabled:      true,
				Issuer:       providerURL,
				ClientID:     "goaway-client",
				ClientSecret: "goaway-secret",
				RedirectURL:  "http://localhost:8080/api/auth/oidc/callback",
				RoleClaim:    "groups",
				AdminGroups:  []string{"dns-admins"},
			},
		},
	}

	a := &API{
		Config:      cfg,
		router:      gin.New(),
		UserService: user.NewService(user.NewRepository(db)),
	}
	a.registerOIDCRoutes()

	// 1. Login redirects to provider and sets oidc_state cookie
	w := httptest.NewRecorder()
	a.router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/auth/oidc/login", nil))
	require.Equal(t, http.StatusFound, w.Code)

	loc, err := url.Parse(w.Header().Get("Location"))
	require.NoError(t, err)
	state := loc.Query().Get("state")
	require.NotEmpty(t, state)

	var stateCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "oidc_state" {
			stateCookie = c
		}
	}
	require.NotNil(t, stateCookie)
	assert.Equal(t, state, stateCookie.Value)

	// 2. Callback with mismatched state fails
	wBad := httptest.NewRecorder()
	badReq := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?state=wrong&code=abc", nil)
	badReq.AddCookie(stateCookie)
	a.router.ServeHTTP(wBad, badReq)
	assert.Equal(t, http.StatusBadRequest, wBad.Code)

	// 3. Valid callback provisions user, maps admin role, and sets jwt cookie
	wOK := httptest.NewRecorder()
	okReq := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?state="+state+"&code=abc", nil)
	okReq.AddCookie(stateCookie)
	a.router.ServeHTTP(wOK, okReq)
	require.Equal(t, http.StatusOK, wOK.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(wOK.Body.Bytes(), &body))
	assert.Equal(t, "carol", body["username"])
	assert.Equal(t, "admin", body["role"])
	assert.True(t, a.UserService.Exists("carol"))

	var jwtCookie *http.Cookie
	for _, c := range wOK.Result().Cookies() {
		if c.Name == "jwt" {
			jwtCookie = c
		}
	}
	require.NotNil(t, jwtCookie)
	assert.NotEmpty(t, jwtCookie.Value)

	// 4. Role mapping falls back to viewer for non-admin groups
	assert.Equal(t, "viewer", mapOIDCRole(map[string]any{"groups": []any{"readonly"}}, "groups", []string{"dns-admins"}))
}
