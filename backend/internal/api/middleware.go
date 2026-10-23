package api

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			b := make([]byte, 8)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
		}
		c.Set("rid", id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

func accessLog(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info("http", "rid", c.GetString("rid"), "method", c.Request.Method, "path", c.FullPath(),
			"status", c.Writer.Status(), "ms", time.Since(start).Milliseconds(), "ip", c.ClientIP())
	}
}

func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer") // deep-link tokens never leak via Referer
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		h.Set("Cache-Control", "no-store")
		c.Next()
	}
}

func cors(origins []string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, o := range origins {
		allowed[strings.TrimSpace(o)] = true
	}
	return func(c *gin.Context) {
		if o := c.GetHeader("Origin"); allowed[o] {
			c.Header("Access-Control-Allow-Origin", o)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// authenticate enforces zero-trust: EVERY API call — including deep-link resolution —
// needs a valid bearer token. A deep link alone never grants access to data.
// Tokens are HS256 JWTs here; swap the keyfunc for your IdP's JWKS (Entra ID, Okta, Keycloak).
func authenticate(secret string, disabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if disabled {
			c.Set("actor", "dev")
			c.Next()
			return
		}
		raw := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if raw == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		tok, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) { return []byte(secret), nil },
			jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithLeeway(30*time.Second))
		if err != nil || !tok.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		claims, _ := tok.Claims.(jwt.MapClaims)
		sub, _ := claims["sub"].(string)
		role, _ := claims["role"].(string)
		c.Set("actor", sub)
		c.Set("role", role)
		c.Next()
	}
}

// requireAdmin guards configuration endpoints (connections, rules, channels).
func requireAdmin(disabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if disabled || c.GetString("role") == "admin" {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin role required"})
	}
}
