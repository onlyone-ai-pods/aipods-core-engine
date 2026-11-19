package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/martinllanos/only-ai-pods/internal/auth"
	"github.com/martinllanos/only-ai-pods/internal/rag"
	"github.com/martinllanos/only-ai-pods/internal/security"
)

// cacheTTL reads a Go duration ("24h", "168h") from the environment; empty or
// invalid values fall back to the default (SPEC-CORE-71.R5).
func cacheTTL(getenv func(string) string, name string, def time.Duration) time.Duration {
	return durationEnv(getenv, name, def)
}

// openRedis connects to REDIS_URL. Without it, it returns nil (process memory
// is used instead), except in production, where it is an error
// (SPEC-CORE-73.R12). The single client is shared by the semantic cache and
// the rate limiter (R7, R8).
func openRedis(getenv func(string) string) (*rag.RedisCacheBackend, error) {
	url := strings.TrimSpace(getenv("REDIS_URL"))
	if url == "" {
		if isProduction(getenv) {
			return nil, errors.New("REDIS_URL is required when ENVIRONMENT=production")
		}
		log.Println("WARNING: REDIS_URL not set; semantic cache and rate limiter live in process memory")
		return nil, nil
	}
	backend, err := rag.NewRedisCacheBackend(url)
	if err != nil {
		return nil, fmt.Errorf("redis: %w", err)
	}
	if err := backend.Ping(context.Background()); err != nil {
		log.Printf("WARNING: Redis not reachable yet; cache misses and the rate limiter fails open until it is: %v", err)
	} else {
		log.Println("redis: backend ready")
	}
	return backend, nil
}

// buildRateLimiter shares the quota through Redis, or keeps it in memory when
// there is no Redis (SPEC-CORE-73.R8).
func buildRateLimiter(redisBackend *rag.RedisCacheBackend) *security.RateLimiter {
	if redisBackend == nil {
		return security.NewRateLimiter(security.NewMemoryCounter(), security.DefaultMaxRPM)
	}
	return security.NewRateLimiter(security.NewRedisCounter(redisBackend.Client()), security.DefaultMaxRPM)
}

// buildSemanticCache uses Redis when it is configured and process memory
// otherwise. Entries are always encrypted with the vault key (SPEC-CORE-71.R5).
func buildSemanticCache(getenv func(string) string, cipher rag.CacheCipher, redisBackend *rag.RedisCacheBackend) (*rag.SemanticCacheManager, error) {
	var backend rag.CacheBackend = rag.NewMemoryCacheBackend()
	if redisBackend != nil {
		backend = redisBackend
	}
	return rag.NewSemanticCacheManager(backend, cipher,
		cacheTTL(getenv, "CACHE_TTL_PROVISIONAL", rag.DefaultTTLProv),
		cacheTTL(getenv, "CACHE_TTL_VERIFIED", rag.DefaultTTLVerified))
}

// feedbackCache is what the feedback endpoint needs from the semantic cache.
type feedbackCache interface {
	Promote(ctx context.Context, tenantID, query string) bool
	PurgeKey(ctx context.Context, tenantID, query string) bool
}

// feedbackHandler serves POST /api/v1/feedback: 👍 promotes the cached answer
// to VERIFIED, 👎 purges it (SPEC-CORE-71.R4).
func feedbackHandler(guard *auth.Guard, cache feedbackCache) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req FeedbackRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.Type != "up" && req.Type != "down" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "type must be 'up' or 'down'"})
			return
		}

		tenantID, ok := guard.ResolveTenant(c, req.TenantID, "GLOBAL", false)
		if !ok {
			return
		}

		purged, promoted := false, false
		if strings.TrimSpace(req.Query) != "" {
			if req.Type == "down" {
				purged = cache.PurgeKey(c.Request.Context(), tenantID, req.Query)
			} else {
				promoted = cache.Promote(c.Request.Context(), tenantID, req.Query)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"status":         "ACCEPTED",
			"feedback_type":  req.Type,
			"reason":         req.Reason,
			"cache_purged":   purged,
			"cache_promoted": promoted,
			"message":        "Feedback registrado exitosamente.",
		})
	}
}
