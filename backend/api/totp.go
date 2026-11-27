package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" // #nosec G505 - RFC 6238 TOTP standard uses HMAC-SHA1
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"goaway/backend/audit"
	"goaway/backend/database"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const totpPeriodSeconds = 30

func generateTOTPSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf), nil
}

// generateTOTPCode computes a 6-digit RFC 6238 TOTP code for the given base32 secret and timestamp.
func generateTOTPCode(secret string, at time.Time) (string, error) {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(secret), " ", ""))
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(normalized)
	if err != nil {
		return "", err
	}

	step := at.Unix() / totpPeriodSeconds
	if step < 0 {
		step = 0
	}
	var msg [8]byte
	// #nosec G115 - step is guaranteed non-negative
	binary.BigEndian.PutUint64(msg[:], uint64(step))

	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	code := (uint32(sum[offset]&0x7f) << 24) |
		(uint32(sum[offset+1]) << 16) |
		(uint32(sum[offset+2]) << 8) |
		uint32(sum[offset+3])

	return fmt.Sprintf("%06d", code%1000000), nil
}

// verifyTOTPCode checks code against the current 30s window and ±1 adjacent step for clock skew.
func verifyTOTPCode(secret, code string, now time.Time) bool {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}
	for _, delta := range []time.Duration{-30 * time.Second, 0, 30 * time.Second} {
		expected, err := generateTOTPCode(secret, now.Add(delta))
		if err == nil && hmac.Equal([]byte(expected), []byte(code)) {
			return true
		}
	}
	return false
}

func (api *API) registerTOTPRoutes() {
	api.routes.GET("/auth/2fa/status", api.handleTOTPStatus)
	api.routes.POST("/auth/2fa/setup", api.handleTOTPSetup)
	api.routes.POST("/auth/2fa/enable", api.handleTOTPEnable)
	api.routes.POST("/auth/2fa/disable", api.handleTOTPDisable)
}

func (api *API) checkUserTOTP(username, code string) (required bool, valid bool) {
	if api.DBConn == nil || username == "" {
		return false, true
	}
	var rec database.UserTOTP
	if err := api.DBConn.Where("username = ? AND enabled = ?", username, true).First(&rec).Error; err != nil {
		return false, true
	}
	return true, verifyTOTPCode(rec.Secret, code, time.Now())
}

func (api *API) currentUsername(c *gin.Context) string {
	if u := c.GetString("username"); u != "" {
		return u
	}
	return "admin"
}

func (api *API) handleTOTPStatus(c *gin.Context) {
	username := api.currentUsername(c)
	enabled := false
	if api.DBConn != nil {
		var rec database.UserTOTP
		if err := api.DBConn.Where("username = ?", username).First(&rec).Error; err == nil {
			enabled = rec.Enabled
		}
	}
	c.JSON(http.StatusOK, gin.H{"enabled": enabled})
}

func (api *API) handleTOTPSetup(c *gin.Context) {
	if api.DBConn == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
		return
	}
	username := api.currentUsername(c)
	secret, err := generateTOTPSecret()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate 2FA secret"})
		return
	}

	var rec database.UserTOTP
	res := api.DBConn.Where("username = ?", username).First(&rec)
	if res.Error != nil {
		rec = database.UserTOTP{Username: username, Secret: secret, Enabled: false}
		if err := api.DBConn.Create(&rec).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store 2FA secret"})
			return
		}
	} else {
		// Re-running setup resets the secret and Enabled; once 2FA is on that would
		// let any session turn it off without the step-up code /disable requires.
		if rec.Enabled {
			c.JSON(http.StatusConflict, gin.H{"error": "2FA is already enabled; disable it first"})
			return
		}
		rec.Secret = secret
		rec.Enabled = false
		if err := api.DBConn.Save(&rec).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update 2FA secret"})
			return
		}
	}

	otpURI := fmt.Sprintf(
		"otpauth://totp/GoAway:%s?secret=%s&issuer=GoAway&algorithm=SHA1&digits=6&period=30",
		url.PathEscape(username),
		secret,
	)
	c.JSON(http.StatusOK, gin.H{"secret": secret, "otpauthUrl": otpURI})
}

func (api *API) handleTOTPEnable(c *gin.Context) {
	if api.DBConn == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
		return
	}
	var input struct {
		Code string `json:"code"`
	}
	if err := c.BindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	username := api.currentUsername(c)
	var rec database.UserTOTP
	if err := api.DBConn.Where("username = ?", username).First(&rec).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "run 2FA setup first"})
		return
	}
	if !verifyTOTPCode(rec.Secret, input.Code, time.Now()) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid 2FA code"})
		return
	}

	rec.Enabled = true
	if err := api.DBConn.Save(&rec).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to enable 2FA"})
		return
	}
	if api.AuditService != nil {
		api.AuditService.CreateAudit(&audit.Entry{
			Topic:     audit.TopicUser,
			Message:   fmt.Sprintf("Enabled TOTP 2FA for user %s", username),
			CreatedAt: time.Now(),
		})
	}
	c.JSON(http.StatusOK, gin.H{"enabled": true})
}

func (api *API) handleTOTPDisable(c *gin.Context) {
	if api.DBConn == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
		return
	}
	var input struct {
		Code string `json:"code"`
	}
	if err := c.BindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	username := api.currentUsername(c)
	var rec database.UserTOTP
	if err := api.DBConn.Where("username = ? AND enabled = ?", username, true).First(&rec).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "2FA is not enabled"})
		return
	}
	if !verifyTOTPCode(rec.Secret, input.Code, time.Now()) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid 2FA step-up code"})
		return
	}

	if err := api.DBConn.Delete(&rec).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to disable 2FA"})
		return
	}
	if api.AuditService != nil {
		api.AuditService.CreateAudit(&audit.Entry{
			Topic:     audit.TopicUser,
			Message:   fmt.Sprintf("Disabled TOTP 2FA for user %s", username),
			CreatedAt: time.Now(),
		})
	}
	c.JSON(http.StatusOK, gin.H{"enabled": false})
}
