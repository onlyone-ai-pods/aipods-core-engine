package rag

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/martinllanos/only-ai-pods/internal/pod"
)

// GlobalTenant is the cache partition shared by every tenant (SPEC-CORE-71.R2).
const GlobalTenant = "GLOBAL"

// Cache scopes and lifecycle states returned to clients (SPEC-CORE-71.R1, R4).
const (
	CacheScopeGlobal   = "GLOBAL"
	CacheScopeTenant   = "TENANT"
	CacheProvisional   = "PROVISIONAL"
	CacheVerified      = "VERIFIED"
	cacheKeyPrefix     = "aipods:semcache:v1:"
	DefaultTTLProv     = 24 * time.Hour
	DefaultTTLVerified = 7 * 24 * time.Hour
)

// CacheBackend stores opaque values with a TTL. Implementations: memory and Redis.
type CacheBackend interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Del(ctx context.Context, key string) (bool, error)
}

// CacheCipher encrypts cached entries at rest. The native vault implements it
// with AES-256-GCM (SPEC-CORE-71, ISO 27001).
type CacheCipher interface {
	EncryptAES256GCM(plainText string) (string, error)
	DecryptAES256GCM(cipherText string) (string, error)
}

// CacheItem is what gets serialized (and encrypted) into the backend.
type CacheItem struct {
	Query     string           `json:"query"`
	TenantID  string           `json:"tenant_id"`
	Status    string           `json:"status"`
	Response  *pod.PodResponse `json:"response"`
	CreatedAt time.Time        `json:"created_at"`
}

// SemanticCacheManager answers repeated questions without the LLM, in two
// partitions: the tenant's own (private) and GLOBAL (shared) (SPEC-CORE-71).
type SemanticCacheManager struct {
	backend        CacheBackend
	cipher         CacheCipher
	ttlProvisional time.Duration
	ttlVerified    time.Duration

	hitCount    atomic.Int64
	missCount   atomic.Int64
	purgedCount atomic.Int64
}

// NewSemanticCacheManager requires a backend and a cipher: entries are never
// stored in clear text. Non-positive TTLs take the defaults (24h / 7 days).
func NewSemanticCacheManager(backend CacheBackend, cipher CacheCipher, ttlProvisional, ttlVerified time.Duration) (*SemanticCacheManager, error) {
	if backend == nil {
		return nil, errors.New("semantic cache: backend is required")
	}
	if cipher == nil {
		return nil, errors.New("semantic cache: cipher is required")
	}
	if ttlProvisional <= 0 {
		ttlProvisional = DefaultTTLProv
	}
	if ttlVerified <= 0 {
		ttlVerified = DefaultTTLVerified
	}
	return &SemanticCacheManager{backend: backend, cipher: cipher, ttlProvisional: ttlProvisional, ttlVerified: ttlVerified}, nil
}

// NormalizeQuery lowercases, removes accents, punctuation at the edges and
// repeated spaces, so trivially different spellings share one cache entry.
func NormalizeQuery(q string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	s, _, err := transform.String(t, q)
	if err != nil {
		s = q
	}
	s = strings.ToLower(strings.Join(strings.Fields(s), " "))
	return strings.TrimFunc(s, func(r rune) bool { return unicode.IsPunct(r) || unicode.IsSpace(r) })
}

// cacheKey never contains the query text: only its SHA-256.
func cacheKey(tenantID, query string) string {
	sum := sha256.Sum256([]byte(NormalizeQuery(query)))
	return cacheKeyPrefix + tenantID + ":" + hex.EncodeToString(sum[:])
}

// partitions lists the keys to look at for a tenant: its own first, then GLOBAL.
func partitions(tenantID, query string) []struct{ key, scope string } {
	out := []struct{ key, scope string }{}
	if tenantID != GlobalTenant {
		out = append(out, struct{ key, scope string }{cacheKey(tenantID, query), CacheScopeTenant})
	}
	return append(out, struct{ key, scope string }{cacheKey(GlobalTenant, query), CacheScopeGlobal})
}

func (c *SemanticCacheManager) load(ctx context.Context, key string) (*CacheItem, bool) {
	raw, ok, err := c.backend.Get(ctx, key)
	if err != nil {
		log.Printf("[semantic-cache] get failed: %v", err)
		return nil, false
	}
	if !ok {
		return nil, false
	}
	plain, err := c.cipher.DecryptAES256GCM(string(raw))
	if err != nil {
		log.Printf("[semantic-cache] decrypt failed, dropping entry: %v", err)
		return nil, false
	}
	var item CacheItem
	if err := json.Unmarshal([]byte(plain), &item); err != nil || item.Response == nil {
		log.Printf("[semantic-cache] corrupt entry, dropping it")
		return nil, false
	}
	return &item, true
}

func (c *SemanticCacheManager) save(ctx context.Context, key string, item *CacheItem, ttl time.Duration) error {
	plain, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("semantic cache: marshal: %w", err)
	}
	sealed, err := c.cipher.EncryptAES256GCM(string(plain))
	if err != nil {
		return fmt.Errorf("semantic cache: encrypt: %w", err)
	}
	if err := c.backend.Set(ctx, key, []byte(sealed), ttl); err != nil {
		return fmt.Errorf("semantic cache: set: %w", err)
	}
	return nil
}

// GetCachedResponse looks in the tenant partition and then in GLOBAL
// (SPEC-CORE-71.R1-R3). The returned copy is marked cached with its scope and status.
func (c *SemanticCacheManager) GetCachedResponse(ctx context.Context, tenantID, query string) (*pod.PodResponse, bool) {
	for _, p := range partitions(tenantID, query) {
		item, ok := c.load(ctx, p.key)
		if !ok {
			continue
		}
		c.hitCount.Add(1)
		out := *item.Response
		out.Cached = true
		out.CacheScope = p.scope
		out.CacheStatus = item.Status
		return &out, true
	}
	c.missCount.Add(1)
	return nil, false
}

// StoreResponse saves a PROVISIONAL answer in the tenant partition, or in
// GLOBAL when global is true (SPEC-CORE-71.R2-R4). Errors are logged, never fatal.
func (c *SemanticCacheManager) StoreResponse(ctx context.Context, tenantID, query string, resp *pod.PodResponse, global bool) {
	if resp == nil || tenantID == "" {
		return
	}
	owner := tenantID
	if global {
		owner = GlobalTenant
	}
	clean := *resp
	clean.Cached, clean.CacheScope, clean.CacheStatus = false, "", ""
	item := &CacheItem{Query: NormalizeQuery(query), TenantID: owner, Status: CacheProvisional, Response: &clean, CreatedAt: time.Now().UTC()}
	if err := c.save(ctx, cacheKey(owner, query), item, c.ttlProvisional); err != nil {
		log.Printf("[semantic-cache] store failed: %v", err)
	}
}

// Promote marks the answer VERIFIED after a 👍 and extends its TTL (SPEC-CORE-71.R4).
func (c *SemanticCacheManager) Promote(ctx context.Context, tenantID, query string) bool {
	for _, p := range partitions(tenantID, query) {
		item, ok := c.load(ctx, p.key)
		if !ok {
			continue
		}
		item.Status = CacheVerified
		if err := c.save(ctx, p.key, item, c.ttlVerified); err != nil {
			log.Printf("[semantic-cache] promote failed: %v", err)
			return false
		}
		return true
	}
	return false
}

// PurgeKey removes the answer after a 👎 from the tenant partition and from
// GLOBAL, so the next question runs the full search (SPEC-CORE-71.R4).
func (c *SemanticCacheManager) PurgeKey(ctx context.Context, tenantID, query string) bool {
	purged := false
	for _, p := range partitions(tenantID, query) {
		ok, err := c.backend.Del(ctx, p.key)
		if err != nil {
			log.Printf("[semantic-cache] purge failed: %v", err)
			continue
		}
		if ok {
			c.purgedCount.Add(1)
			purged = true
		}
	}
	return purged
}

// Stats returns hits, misses and purged entries since start.
func (c *SemanticCacheManager) Stats() (int64, int64, int64) {
	return c.hitCount.Load(), c.missCount.Load(), c.purgedCount.Load()
}

// MemoryCacheBackend keeps entries in process memory (no REDIS_URL, tests).
type MemoryCacheBackend struct {
	mu    sync.Mutex
	items map[string]memoryEntry
	now   func() time.Time
}

type memoryEntry struct {
	value   []byte
	expires time.Time
}

func NewMemoryCacheBackend() *MemoryCacheBackend {
	return &MemoryCacheBackend{items: make(map[string]memoryEntry), now: time.Now}
}

func (m *MemoryCacheBackend) Get(_ context.Context, key string) ([]byte, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.items[key]
	if !ok {
		return nil, false, nil
	}
	if m.now().After(e.expires) {
		delete(m.items, key)
		return nil, false, nil
	}
	return e.value, true, nil
}

func (m *MemoryCacheBackend) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[key] = memoryEntry{value: value, expires: m.now().Add(ttl)}
	return nil
}

func (m *MemoryCacheBackend) Del(_ context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.items[key]
	delete(m.items, key)
	return ok, nil
}
