package security

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/martinllanos/only-ai-pods/internal/tenant"
	"github.com/redis/go-redis/v9"
)

func newMiniredisClient(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return mr, client
}

func TestRateLimiter_MemoryAllow(t *testing.T) {
	limiter := NewRateLimiter(NewMemoryCounter(), 3)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		allowed, rem, err := limiter.Allow(ctx, "tenant:test")
		if !allowed || err != nil {
			t.Fatalf("request %d should be allowed (err=%v)", i+1, err)
		}
		if rem != 2-i {
			t.Errorf("remaining = %d, want %d", rem, 2-i)
		}
	}
	if allowed, rem, _ := limiter.Allow(ctx, "tenant:test"); allowed || rem != 0 {
		t.Fatalf("4th request: allowed=%t remaining=%d, want blocked", allowed, rem)
	}
}

func TestRateLimiter_MemoryWindowResets(t *testing.T) {
	now := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
	counter := NewMemoryCounter()
	counter.now = func() time.Time { return now }
	limiter := NewRateLimiter(counter, 1)
	limiter.now = counter.now
	ctx := context.Background()
	if ok, _, _ := limiter.Allow(ctx, "ip:1.2.3.4"); !ok {
		t.Fatal("first request blocked")
	}
	if ok, _, _ := limiter.Allow(ctx, "ip:1.2.3.4"); ok {
		t.Fatal("second request in the same minute allowed")
	}
	now = now.Add(time.Minute)
	if ok, _, _ := limiter.Allow(ctx, "ip:1.2.3.4"); !ok {
		t.Fatal("request in the next window blocked")
	}
}

// Two limiters (blue and green) on the same Redis share the quota: request
// 301 of the minute gets 429 wherever it lands.
func TestSPEC_CORE_73_R8_RedisQuotaIsSharedBetweenInstances(t *testing.T) {
	_, client := newMiniredisClient(t)
	blue := NewRateLimiter(NewRedisCounter(client), DefaultMaxRPM)
	green := NewRateLimiter(NewRedisCounter(client), DefaultMaxRPM)
	fixed := time.Date(2026, 11, 19, 10, 0, 30, 0, time.UTC)
	blue.now = func() time.Time { return fixed }
	green.now = blue.now
	ctx := context.Background()

	for i := 0; i < DefaultMaxRPM; i++ {
		l := blue
		if i%2 == 1 {
			l = green
		}
		if ok, _, err := l.Allow(ctx, "tenant:acme"); !ok || err != nil {
			t.Fatalf("request %d blocked (err=%v)", i+1, err)
		}
	}
	if ok, rem, err := green.Allow(ctx, "tenant:acme"); ok || rem != 0 || err != nil {
		t.Fatalf("request 301: allowed=%t remaining=%d err=%v, want 429", ok, rem, err)
	}
	if ok, _, _ := blue.Allow(ctx, "tenant:other"); !ok {
		t.Fatal("another tenant must have its own quota")
	}
}

func TestSPEC_CORE_73_R8_RedisKeyHasTTL(t *testing.T) {
	mr, client := newMiniredisClient(t)
	l := NewRateLimiter(NewRedisCounter(client), 10)
	l.now = func() time.Time { return time.Unix(120*60, 0) }
	if _, _, err := l.Allow(context.Background(), "tenant:acme"); err != nil {
		t.Fatal(err)
	}
	key := "aipods:ratelimit:v1:tenant:acme:120"
	if !mr.Exists(key) {
		t.Fatalf("key %q not found; keys: %v", key, mr.Keys())
	}
	if ttl := mr.TTL(key); ttl <= 0 || ttl > 2*time.Minute {
		t.Fatalf("ttl = %s", ttl)
	}
}

type failingCounter struct{}

func (failingCounter) Incr(context.Context, string, time.Duration) (int64, error) {
	return 0, errors.New("redis down")
}

func serve(t *testing.T, l *RateLimiter, tenantID, header string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if tenantID != "" {
		r.Use(func(c *gin.Context) {
			c.Request = c.Request.WithContext(tenant.WithTenantID(c.Request.Context(), tenantID))
		})
	}
	r.Use(l.Middleware())
	r.GET("/api/v1/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	if header != "" {
		req.Header.Set("X-Tenant-ID", header)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSPEC_CORE_73_R8_FailOpenWhenRedisIsDown(t *testing.T) {
	if w := serve(t, NewRateLimiter(failingCounter{}, 1), "acme", ""); w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (fail-open)", w.Code)
	}
}

func TestSPEC_CORE_73_R8_TenantHeaderDoesNotChangeTheKey(t *testing.T) {
	l := NewRateLimiter(NewMemoryCounter(), 1)
	if w := serve(t, l, "", "tenant_a"); w.Code != http.StatusOK {
		t.Fatalf("first: %d", w.Code)
	}
	// A different X-Tenant-ID from the same IP must not get a fresh quota.
	if w := serve(t, l, "", "tenant_b"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("second with another X-Tenant-ID: %d, want 429", w.Code)
	}
	// The authenticated tenant is its own subject.
	if w := serve(t, l, "acme", "tenant_a"); w.Code != http.StatusOK {
		t.Fatalf("authenticated tenant: %d", w.Code)
	}
	if w := serve(t, l, "acme", ""); w.Code != http.StatusTooManyRequests {
		t.Fatalf("authenticated tenant second request: %d, want 429", w.Code)
	}
}
