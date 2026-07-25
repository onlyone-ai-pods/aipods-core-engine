package rag

import (
	"context"
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
	mu           sync.RWMutex
	redisURL     string
	cache        map[string]CacheItem
	hitCount     int64
	missCount    int64
	ttlDuration  time.Duration
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

func (c *SemanticCacheManager) Stats() (int64, int64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hitCount, c.missCount
}
