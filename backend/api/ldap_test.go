package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"goaway/backend/api/ratelimit"
	"goaway/backend/database"
	"goaway/backend/settings"
	"goaway/backend/user"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestBuildLDAPFilterEscapesInjection(t *testing.T) {
	f := buildLDAPFilter("", "admin)(|(uid=*")
	assert.NotContains(t, f, "admin)(|")
	assert.Contains(t, f, `\29`)
	assert.Contains(t, f, `\2a`)
}

func TestMapLDAPRole(t *testing.T) {
	groups := []string{"CN=DNS-Admins,OU=Groups,DC=corp,DC=local"}
	assert.Equal(t, "admin", mapLDAPRole(groups, []string{"dns-admins"}))
	assert.Equal(t, "admin", mapLDAPRole(groups, []string{"CN=DNS-Admins,OU=Groups,DC=corp,DC=local"}))
	assert.Equal(t, "viewer", mapLDAPRole(groups, []string{"network-ops"}))
	assert.Equal(t, "admin", mapLDAPRole(nil, nil))
}

func TestLDAPLoginEmergencyLocalFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&database.User{}))

	userSvc := user.NewService(user.NewRepository(db))
	require.NoError(t, userSvc.CreateUser("admin", "emergency-pass", "admin"))

	jwtSecret := base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	cfg := &settings.Config{
		API: settings.APIConfig{
			JWTSecret: jwtSecret,
			LDAP: settings.LDAPConfig{
				Enabled:       true,
				URL:           "ldap://127.0.0.1:1", // intentionally unreachable
				BaseDN:        "dc=example,dc=com",
				LocalFallback: true,
			},
		},
	}

	a := &API{
		Config:      cfg,
		router:      gin.New(),
		UserService: userSvc,
		RateLimiter: ratelimit.NewRateLimiter(false, 10, 1),
	}
	a.router.POST("/api/login", a.handleLogin)

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "emergency-pass"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	a.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}
