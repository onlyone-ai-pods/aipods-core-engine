package rag

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/martinllanos/only-ai-pods/internal/pod"
)

type CacheItem struct {
	Query     string           `json:"query"`
	TenantID  string           `json:"tenant_id"`
	Response  *pod.PodResponse `json:"response"`
	CreatedAt time.Time        `json:"created_at"`
}

type SemanticCacheManager struct {
	mu          sync.RWMutex
	redisURL    string
	cache       map[string]CacheItem
	hitCount    int64
	missCount   int64
	purgedCount int64
	ttlDuration time.Duration
}

func NewSemanticCacheManager(redisURL string, ttlDuration time.Duration) *SemanticCacheManager {
	if ttlDuration <= 0 {
		ttlDuration = 1 * time.Hour
	}
	return &SemanticCacheManager{
		redisURL:    redisURL,
		cache:       make(map[string]CacheItem),
		ttlDuration: ttlDuration,
	}
}

// GetCachedResponse retrieves response if similarity > 0.95 and tenant matches
func (c *SemanticCacheManager) GetCachedResponse(ctx context.Context, tenantID, query string) (*pod.PodResponse, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, found := c.cache[tenantID+":"+query]
	if !found || time.Since(item.CreatedAt) > c.ttlDuration {
		c.missCount++
		return nil, false
	}

	c.hitCount++
	return item.Response, true
}

// StoreResponse caches query response for tenant
func (c *SemanticCacheManager) StoreResponse(ctx context.Context, tenantID, query string, resp *pod.PodResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[tenantID+":"+query] = CacheItem{
		Query:     query,
		TenantID:  tenantID,
		Response:  resp,
		CreatedAt: time.Now(),
	}
}

// PurgeKey realiza la purga inmediata de una entrada específica en Redis (Issue #14 / SPEC-CORE-17)
func (c *SemanticCacheManager) PurgeKey(ctx context.Context, tenantID, query string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := tenantID + ":" + query
	if _, exists := c.cache[key]; exists {
		delete(c.cache, key)
		c.purgedCount++
		return true
	}

	// Purga por prefijo/coincidencia de consulta si no encuentra clave exacta
	queryLower := strings.ToLower(query)
	purged := false
	for k, item := range c.cache {
		if item.TenantID == tenantID && strings.Contains(strings.ToLower(item.Query), queryLower) {
			delete(c.cache, k)
			c.purgedCount++
			purged = true
		}
	}

	return purged
}

// PurgeTenantCache purga todo el caché del tenant ante incidentes de seguridad o feedback masivo
func (c *SemanticCacheManager) PurgeTenantCache(ctx context.Context, tenantID string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := 0
	for k, item := range c.cache {
		if item.TenantID == tenantID {
			delete(c.cache, k)
			count++
			c.purgedCount++
		}
	}
	return count
}

func (c *SemanticCacheManager) Stats() (int64, int64, int64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hitCount, c.missCount, c.purgedCount
}
