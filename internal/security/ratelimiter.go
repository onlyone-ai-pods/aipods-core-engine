package security

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiterConfig define cuotas por Tenant (RPM / TPM) - SPEC-CORE-07
type RateLimiterConfig struct {
	MaxRPM int           // Requests Per Minute
	Window time.Duration // Ventana de tiempo (ej. 1 minuto)
}

type TokenBucket struct {
	tokens     int
	lastRefill time.Time
}

// RedisRateLimiter gestiona rate limiting por IP y Tenant (Issue #2)
type RedisRateLimiter struct {
	mu        sync.Mutex
	buckets   map[string]*TokenBucket
	config    RateLimiterConfig
	redisAddr string
}

func NewRedisRateLimiter(redisAddr string, maxRPM int) *RedisRateLimiter {
	if maxRPM <= 0 {
		maxRPM = 60 // 60 RPM por defecto
	}
	return &RedisRateLimiter{
		buckets:   make(map[string]*TokenBucket),
		config:    RateLimiterConfig{MaxRPM: maxRPM, Window: 1 * time.Minute},
		redisAddr: redisAddr,
	}
}

// Allow verifica si la clave (TenantID o ClientIP) sobrepasó su cuota
func (r *RedisRateLimiter) Allow(key string) (bool, int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	tb, exists := r.buckets[key]

	if !exists {
		r.buckets[key] = &TokenBucket{
			tokens:     r.config.MaxRPM - 1,
			lastRefill: now,
		}
		return true, r.config.MaxRPM - 1
	}

	// Refill de tokens basado en tiempo transcurrido
	elapsed := now.Sub(tb.lastRefill)
	if elapsed >= r.config.Window {
		tb.tokens = r.config.MaxRPM
		tb.lastRefill = now
	}

	if tb.tokens <= 0 {
		return false, 0
	}

	tb.tokens--
	return true, tb.tokens
}

// Middleware Gin para Rate Limiting (Issue #2)
func (r *RedisRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetHeader("X-Tenant-ID")
		if tenantID == "" {
			tenantID = c.ClientIP()
		}

		key := fmt.Sprintf("ratelimit:%s", tenantID)
		allowed, remaining := r.Allow(key)

		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", r.config.MaxRPM))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

		if !allowed {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded (HTTP 429)",
				"message":     "Ha superado la cuota de peticiones por minuto de su plan. Reintente en un momento.",
				"tenant_id":   tenantID,
				"retry_after": 60,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
