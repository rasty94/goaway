package api

import (
	"goaway/backend/settings"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestReadOnlyMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &settings.Config{API: settings.APIConfig{ReadOnly: true}}
	a := &API{Config: cfg}

	r := gin.New()
	grp := r.Group("/api", a.readOnlyMiddleware())
	grp.GET("/settings", func(c *gin.Context) { c.Status(http.StatusOK) })
	grp.POST("/settings", func(c *gin.Context) { c.Status(http.StatusOK) })
	grp.DELETE("/upstream", func(c *gin.Context) { c.Status(http.StatusOK) })
	grp.POST("/login", func(c *gin.Context) { c.Status(http.StatusOK) })

	// GET allowed
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/settings", nil))
	assert.Equal(t, http.StatusOK, w.Code)

	// Login allowed
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/login", nil))
	assert.Equal(t, http.StatusOK, w.Code)

	// Mutations blocked with 403
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/settings", nil))
	assert.Equal(t, http.StatusForbidden, w.Code)

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/upstream", nil))
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Disabling readOnly allows mutations again
	cfg.LockConfig()
	cfg.API.ReadOnly = false
	cfg.UnlockConfig()

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/settings", nil))
	assert.Equal(t, http.StatusOK, w.Code)
}
