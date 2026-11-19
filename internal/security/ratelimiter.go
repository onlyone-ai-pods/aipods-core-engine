package security

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/martinllanos/only-ai-pods/internal/telemetry"
	"github.com/martinllanos/only-ai-pods/internal/tenant"
	"github.com/redis/go-redis/v9"
)

// DefaultMaxRPM is the per-subject quota when none is configured (SPEC-CORE-07).
const DefaultMaxRPM = 300

// rateLimitWindow is the fixed window of the limiter (SPEC-CORE-73.R8).
const rateLimitWindow = time.Minute

// redisLimiterTimeout bounds each Redis call so a slow Redis never stalls a request.
const redisLimiterTimeout = 200 * time.Millisecond

// Counter counts hits of a key inside a window. Implementations must be safe
// for concurrent use.
type Counter interface {
	// Incr adds one hit to key and returns the hits counted so far. The key
	// expires after ttl.
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
}

// RedisCounter shares the quota between every core instance, blue and green
// (SPEC-CORE-73.R8). The client is owned by the caller.
type RedisCounter struct {
	client redis.UniversalClient
}

func NewRedisCounter(client redis.UniversalClient) *RedisCounter {
	return &RedisCounter{client: client}
}

// Incr runs INCR and EXPIRE in one MULTI/EXEC transaction.
func (r *RedisCounter) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, redisLimiterTimeout)
	defer cancel()
	var incr *redis.IntCmd
	_, err := r.client.TxPipelined(ctx, func(p redis.Pipeliner) error {
		incr = p.Incr(ctx, key)
		p.Expire(ctx, key, ttl)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

// MemoryCounter keeps the counts in process memory: each instance has its own
// quota. Used only outside production without REDIS_URL.
type MemoryCounter struct {
	mu      sync.Mutex
	entries map[string]memoryEntry
	now     func() time.Time
}

type memoryEntry struct {
	count   int64
	expires time.Time
}

func NewMemoryCounter() *MemoryCounter {
	return &MemoryCounter{entries: make(map[string]memoryEntry), now: time.Now}
}

func (m *MemoryCounter) Incr(_ context.Context, key string, ttl time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	// Window keys are short-lived; drop the expired ones so the map stays small.
	if len(m.entries) > 10000 {
		for k, e := range m.entries {
			if now.After(e.expires) {
				delete(m.entries, k)
			}
		}
	}
	e := m.entries[key]
	if now.After(e.expires) {
		e = memoryEntry{}
	}
	e.count++
	e.expires = now.Add(ttl)
	m.entries[key] = e
	return e.count, nil
}

// RateLimiter applies a fixed one-minute window per subject (SPEC-CORE-07,
// SPEC-CORE-73.R8).
type RateLimiter struct {
	counter Counter
	maxRPM  int
	now     func() time.Time
}

func NewRateLimiter(counter Counter, maxRPM int) *RateLimiter {
	if maxRPM <= 0 {
		maxRPM = DefaultMaxRPM
	}
	return &RateLimiter{counter: counter, maxRPM: maxRPM, now: time.Now}
}

// MaxRPM returns the configured quota.
func (r *RateLimiter) MaxRPM() int { return r.maxRPM }

// windowKey is aipods:ratelimit:v1:<subject>:<window start in unix minutes>.
func (r *RateLimiter) windowKey(subject string) string {
	return fmt.Sprintf("aipods:ratelimit:v1:%s:%d", subject, r.now().Unix()/int64(rateLimitWindow/time.Second))
}

// Allow counts one request of subject. When the counter fails the request is
// allowed (fail-open) and the error is returned for logging.
func (r *RateLimiter) Allow(ctx context.Context, subject string) (bool, int, error) {
	n, err := r.counter.Incr(ctx, r.windowKey(subject), rateLimitWindow+5*time.Second)
	if err != nil {
		return true, r.maxRPM, err
	}
	remaining := r.maxRPM - int(n)
	if remaining < 0 {
		return false, 0, nil
	}
	return true, remaining, nil
}

// Subject identifies who consumes the quota: the authenticated tenant, or the
// client IP. The X-Tenant-ID header is ignored because the client controls it.
func Subject(c *gin.Context) string {
	if id, err := tenant.FromContext(c.Request.Context()); err == nil {
		return "tenant:" + id
	}
	return "ip:" + c.ClientIP()
}

// Middleware is the Gin rate limiting middleware (Issue #2).
func (r *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip CORS preflight and system routes.
		path := c.Request.URL.Path
		if c.Request.Method == http.MethodOptions || path == "/healthz" || path == "/livez" || path == "/readyz" ||
			path == "/metrics" || strings.HasPrefix(path, "/api/v1/admin") {
			c.Next()
			return
		}

		subject := Subject(c)
		allowed, remaining, err := r.Allow(c.Request.Context(), subject)
		if err != nil {
			log.Printf("ratelimit: counter unavailable, allowing request (fail-open): %v", err)
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(r.maxRPM))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))

		if !allowed {
			telemetry.GetMetricsCollector().RecordRateLimitExceeded()
			body := gin.H{
				"error":       "Rate limit exceeded (HTTP 429)",
				"message":     "Ha superado la cuota de peticiones por minuto de su plan. Reintente en un momento.",
				"retry_after": 60,
			}
			if id, ok := strings.CutPrefix(subject, "tenant:"); ok {
				body["tenant_id"] = id
			}
			c.JSON(http.StatusTooManyRequests, body)
			c.Abort()
			return
		}
		c.Next()
	}
}
