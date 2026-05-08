// Package middleware provides Gin middleware: auth, rate limiting, tracing, logging.
package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/bzyfuzy/bzy-personal/apps/bzy-interface/internal/auth"
	pkgerrors "github.com/bzyfuzy/bzy-personal/pkg/errors"
)

// Bundle is all middleware collected for DI.
type Bundle struct {
	jwt     *auth.JWTService
	apiKeys *auth.APIKeyService
	limiter *rate.Limiter
	logger  *zap.Logger
}

func New(jwt *auth.JWTService, apiKeys *auth.APIKeyService, rps, burst int, logger *zap.Logger) *Bundle {
	return &Bundle{
		jwt:     jwt,
		apiKeys: apiKeys,
		limiter: rate.NewLimiter(rate.Limit(rps), burst),
		logger:  logger,
	}
}

// ─── Authentication ───────────────────────────────────────────────────────────

// RequireAuth validates Bearer JWT or X-API-Key and sets "claims" in context.
func (b *Bundle) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try Bearer token first
		if token := extractBearer(c); token != "" {
			claims, err := b.jwt.Validate(token)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
				return
			}
			c.Set("claims", claims)
			c.Set("user_id", claims.UserID)
			c.Next()
			return
		}

		// Fallback to API key
		if apiKey := c.GetHeader("X-API-Key"); apiKey != "" {
			userID, ok := b.apiKeys.Validate(apiKey)
			if !ok {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
				return
			}
			c.Set("user_id", userID)
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
	}
}

// ─── Rate limiting ────────────────────────────────────────────────────────────

func (b *Bundle) RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !b.limiter.Allow() {
			err := pkgerrors.RateLimited("too many requests")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    err.Code,
				"message": err.Message,
			})
			return
		}
		c.Next()
	}
}

// ─── Request logging ──────────────────────────────────────────────────────────

func (b *Bundle) RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		b.logger.Info("request",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
		)
	}
}

// ─── OpenTelemetry tracing ────────────────────────────────────────────────────

func (b *Bundle) Tracing(serviceName string) gin.HandlerFunc {
	tracer := otel.Tracer(serviceName)
	return func(c *gin.Context) {
		ctx, span := tracer.Start(c.Request.Context(), c.Request.Method+" "+c.FullPath())
		defer span.End()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// ─── Error handler ────────────────────────────────────────────────────────────

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 {
			return
		}
		err := c.Errors.Last().Err
		if de, ok := pkgerrors.As(err); ok {
			c.JSON(de.HTTPStatus(), de)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}

// ─── CORS ─────────────────────────────────────────────────────────────────────

func CORS(origins []string) gin.HandlerFunc {
	originSet := make(map[string]bool, len(origins))
	for _, o := range origins {
		originSet[o] = true
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if originSet[origin] || originSet["*"] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS,PATCH")
			c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type,X-API-Key")
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func extractBearer(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		return header[7:]
	}
	return ""
}
