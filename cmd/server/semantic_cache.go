package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/martinllanos/only-ai-pods/internal/auth"
	"github.com/martinllanos/only-ai-pods/internal/rag"
)

// cacheTTL reads a Go duration ("24h", "168h") from the environment; empty or
// invalid values fall back to the default (SPEC-CORE-71.R5).
func cacheTTL(getenv func(string) string, name string, def time.Duration) time.Duration {
	raw := strings.TrimSpace(getenv(name))
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		log.Printf("WARNING: %s=%q is not a valid duration; using %s", name, raw, def)
		return def
	}
	return d
}

// buildSemanticCache uses Redis when REDIS_URL is set and process memory
// otherwise. Entries are always encrypted with the vault key (SPEC-CORE-71.R5).
func buildSemanticCache(getenv func(string) string, cipher rag.CacheCipher) (*rag.SemanticCacheManager, error) {
	var backend rag.CacheBackend = rag.NewMemoryCacheBackend()
	if url := strings.TrimSpace(getenv("REDIS_URL")); url != "" {
		redisBackend, err := rag.NewRedisCacheBackend(url)
		if err != nil {
			return nil, fmt.Errorf("semantic cache: %w", err)
		}
		if err := redisBackend.Ping(context.Background()); err != nil {
			log.Printf("WARNING: semantic cache: Redis not reachable yet, cache misses until it is: %v", err)
		} else {
			log.Println("semantic cache: Redis backend ready")
		}
		backend = redisBackend
	} else {
		log.Println("semantic cache: REDIS_URL not set; using in-memory cache")
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
