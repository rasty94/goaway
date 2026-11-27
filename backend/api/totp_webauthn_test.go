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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestTOTPGenerateVerifyAndLoginEnforcement(t *testing.T) {
	gin.SetMode(gin.TestMode)

	secret, err := generateTOTPSecret()
	require.NoError(t, err)
	now := time.Now()
	code, err := generateTOTPCode(secret, now)
	require.NoError(t, err)
	assert.Len(t, code, 6)
	assert.True(t, verifyTOTPCode(secret, code, now))
	assert.True(t, verifyTOTPCode(secret, code, now.Add(25*time.Second)))
	assert.False(t, verifyTOTPCode(secret, "000000", now))

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&database.User{}, &database.UserTOTP{}, &database.WebAuthnCredential{}))

	userSvc := user.NewService(user.NewRepository(db))
	require.NoError(t, userSvc.CreateUser("admin", "pass1234", "admin"))

	jwtSecret := base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	cfg := &settings.Config{
		API: settings.APIConfig{
			JWTSecret: jwtSecret,
			WebAuthn:  settings.WebAuthnConfig{Enabled: true, RPID: "localhost"},
		},
	}

	a := &API{
		Config:      cfg,
		DBConn:      db,
		router:      gin.New(),
		UserService: userSvc,
		RateLimiter: ratelimit.NewRateLimiter(false, 10, 1),
	}
	a.routes = a.router.Group("/api")
	a.registerAuthRoutes()

	// Setup 2FA
	wSetup := httptest.NewRecorder()
	a.router.ServeHTTP(wSetup, httptest.NewRequest(http.MethodPost, "/api/auth/2fa/setup", nil))
	require.Equal(t, http.StatusOK, wSetup.Code)
	var setupResp map[string]string
	require.NoError(t, json.Unmarshal(wSetup.Body.Bytes(), &setupResp))
	require.NotEmpty(t, setupResp["secret"])
	assert.Contains(t, setupResp["otpauthUrl"], "otpauth://totp/GoAway:admin")

	// Enable 2FA with valid TOTP code
	liveCode, err := generateTOTPCode(setupResp["secret"], time.Now())
	require.NoError(t, err)
	enableBody, _ := json.Marshal(map[string]string{"code": liveCode})
	wEnable := httptest.NewRecorder()
	reqEnable := httptest.NewRequest(http.MethodPost, "/api/auth/2fa/enable", bytes.NewReader(enableBody))
	reqEnable.Header.Set("Content-Type", "application/json")
	a.router.ServeHTTP(wEnable, reqEnable)
	require.Equal(t, http.StatusOK, wEnable.Code)

	// Setup must not reset an enabled 2FA (that would bypass the /disable step-up code)
	wResetup := httptest.NewRecorder()
	a.router.ServeHTTP(wResetup, httptest.NewRequest(http.MethodPost, "/api/auth/2fa/setup", nil))
	require.Equal(t, http.StatusConflict, wResetup.Code)

	// Login without 2FA code must return 401 with requires2FA=true
	no2faBody, _ := json.Marshal(map[string]string{"username": "admin", "password": "pass1234"})
	wNo2FA := httptest.NewRecorder()
	reqNo2FA := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(no2faBody))
	reqNo2FA.Header.Set("Content-Type", "application/json")
	a.router.ServeHTTP(wNo2FA, reqNo2FA)
	require.Equal(t, http.StatusUnauthorized, wNo2FA.Code)
	assert.Contains(t, wNo2FA.Body.String(), `"requires2FA":true`)

	// Login with valid 2FA code succeeds
	with2faBody, _ := json.Marshal(map[string]string{"username": "admin", "password": "pass1234", "totpCode": liveCode})
	wWith2FA := httptest.NewRecorder()
	reqWith2FA := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(with2faBody))
	reqWith2FA.Header.Set("Content-Type", "application/json")
	a.router.ServeHTTP(wWith2FA, reqWith2FA)
	require.Equal(t, http.StatusOK, wWith2FA.Code)

	// WebAuthn registration begin returns challenge + sessionId
	wReg := httptest.NewRecorder()
	reqReg := httptest.NewRequest(http.MethodPost, "/api/auth/webauthn/register/begin", bytes.NewReader([]byte(`{"name":"MacBook"}`)))
	reqReg.Header.Set("Content-Type", "application/json")
	a.router.ServeHTTP(wReg, reqReg)
	require.Equal(t, http.StatusOK, wReg.Code)
	assert.Contains(t, wReg.Body.String(), `"sessionId"`)
	assert.Contains(t, wReg.Body.String(), `"challenge"`)

	// The client feeds options.publicKey to PublicKeyCredential.parseCreationOptionsFromJSON.
	var regResp struct {
		Options struct {
			PublicKey struct {
				Challenge string `json:"challenge"`
				User      struct {
					Name string `json:"name"`
				} `json:"user"`
			} `json:"publicKey"`
		} `json:"options"`
	}
	require.NoError(t, json.Unmarshal(wReg.Body.Bytes(), &regResp))
	assert.NotEmpty(t, regResp.Options.PublicKey.Challenge)
	assert.Equal(t, "admin", regResp.Options.PublicKey.User.Name)
}
