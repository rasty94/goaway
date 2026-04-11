package api

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"goaway/backend/audit"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	tokenDuration = 5 * time.Minute
)

func (api *API) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/server") {
			c.Next()
			return
		}

		if apiKey := c.GetHeader("api-key"); apiKey != "" {
			if api.KeyService.VerifyKeyScope(apiKey, "read") {
				c.Set("is_api_key", true)
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key or insufficient scopes"})
			return
		}

		cookie, err := c.Cookie("jwt")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing authorization cookie"})
			return
		}

		claims, err := parseToken(cookie, api.Config.API.JWTSecret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		username, ok := claims["username"].(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			return
		}

		now := time.Now().Unix()
		exp, ok := claims["exp"].(float64)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token expiration"})
			return
		}
		expiration := int64(exp)

		if now >= expiration {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token expired"})
			return
		}

		halfDurationSeconds := int64(tokenDuration.Seconds() / 2)
		timeUntilExpiration := expiration - now

		if timeUntilExpiration <= halfDurationSeconds {
			newToken, err := generateToken(username, api.Config.API.JWTSecret)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to renew token"})
				return
			}
			setAuthCookie(c.Writer, newToken)
			log.Debug("New token generated and cookie set")
		}

		c.Set("username", username)
		c.Next()
	}
}

func (api *API) auditMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "GET" || c.Request.Method == "OPTIONS" || c.Request.Method == "HEAD" {
			c.Next()
			return
		}

		c.Next()

		if c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
			username := c.GetString("username")
			if username == "" && c.GetBool("is_api_key") {
				username = "APIKey"
			}
			if username == "" {
				username = "system"
			}

			api.AuditService.CreateAudit(&audit.Entry{
				Topic:   audit.TopicAPI,
				Message: fmt.Sprintf("User %s performed %s on %s", username, c.Request.Method, c.Request.URL.Path),
			})
		}
	}
}

func parseToken(tokenString string, b64secret string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		secret, err := base64.RawURLEncoding.DecodeString(b64secret)
		if err != nil {
			return "", err
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

func generateToken(username string, b64secret string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"username": username,
		"exp":      now.Add(tokenDuration).Unix(),
		"iat":      now.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	secret, err := base64.RawURLEncoding.DecodeString(b64secret)
	if err != nil {
		return "", err
	}
	return token.SignedString([]byte(secret))
}

func setAuthCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true, // Required by auditors
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(tokenDuration),
		MaxAge:   int(tokenDuration.Seconds()),
	})
}
