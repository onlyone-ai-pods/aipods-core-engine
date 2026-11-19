package main

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/martinllanos/only-ai-pods/internal/store"
)

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestSPEC_CORE_73_R12_ProductionRequiresDatabaseURL(t *testing.T) {
	_, err := buildStore(context.Background(), envOf(map[string]string{"ENVIRONMENT": "production"}))
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("err = %v, want DATABASE_URL required", err)
	}
	st, err := buildStore(context.Background(), envOf(map[string]string{"ENVIRONMENT": "development"}))
	if err != nil || st.Name() != "memory" {
		t.Fatalf("outside production memory is allowed: st=%v err=%v", st, err)
	}
}

func TestSPEC_CORE_73_R12_ProductionRequiresRedisURL(t *testing.T) {
	_, err := openRedis(envOf(map[string]string{"ENVIRONMENT": "Production"}))
	if err == nil || !strings.Contains(err.Error(), "REDIS_URL") {
		t.Fatalf("err = %v, want REDIS_URL required", err)
	}
	b, err := openRedis(envOf(map[string]string{}))
	if err != nil || b != nil {
		t.Fatalf("outside production no Redis is allowed: b=%v err=%v", b, err)
	}
	if l := buildRateLimiter(nil); l.MaxRPM() != 300 {
		t.Fatalf("max rpm = %d, want 300", l.MaxRPM())
	}
}

// emptyCountStore always reports an empty users table, as both colors see it
// when they start at the same time.
type emptyCountStore struct{ store.Store }

func (emptyCountStore) CountUsers(context.Context) (int, error) { return 0, nil }

func TestSPEC_CORE_73_R13_BootstrapAdminIsIdempotent(t *testing.T) {
	ctx := context.Background()
	mem := store.NewMemory()
	st := emptyCountStore{mem}
	env := envOf(map[string]string{"AUTH_BOOTSTRAP_EMAIL": "admin@example.com", "AUTH_BOOTSTRAP_PASSWORD": "a-long-test-password"})

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- bootstrapAdmin(ctx, st, env)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("bootstrapAdmin: %v", err)
		}
	}
	if n, err := mem.CountUsers(ctx); err != nil || n != 1 {
		t.Fatalf("users = %d (err %v), want 1", n, err)
	}
}
