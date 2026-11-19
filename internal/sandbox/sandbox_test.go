package sandbox

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/martinllanos/only-ai-pods/internal/router"
	"github.com/redis/go-redis/v9"
)

func TestSandboxSessionManager(t *testing.T) {
	smartRouter := router.NewDynamicSmartRouter()
	mgr := NewSessionManager(smartRouter)
	ctx := context.Background()

	session, err := mgr.CreateEphemeralSession(ctx, "manual_afip_test.pdf")
	if err != nil || session.SessionID == "" {
		t.Fatalf("Expected valid SessionID, got %+v (%v)", session, err)
	}

	for i, q := range []string{"Cómo genero mi archivo CSR para AFIP?", "Consulta 2 de prueba", "Consulta 3 de prueba"} {
		res, s, err := mgr.ExecuteSandboxQuery(ctx, session.SessionID, q)
		if err != nil || res == nil {
			t.Fatalf("Query %d failed: %v", i+1, err)
		}
		if s.QueryCount != i+1 {
			t.Errorf("Expected QueryCount = %d, got %d", i+1, s.QueryCount)
		}
	}

	// Limit test when QueryCount reaches MaxQueries
	stored, _, _ := mgr.store.Get(ctx, session.SessionID)
	stored.QueryCount = stored.MaxQueries
	if err := mgr.store.Save(ctx, stored); err != nil {
		t.Fatal(err)
	}
	if _, _, err := mgr.ExecuteSandboxQuery(ctx, session.SessionID, "Consulta no permitida al superar el límite"); err != ErrMaxQueriesExceeded {
		t.Errorf("Expected ErrMaxQueriesExceeded, got: %v", err)
	}
}

func TestSandboxUnknownSessionIsAutoCreated(t *testing.T) {
	mgr := NewSessionManager(router.NewDynamicSmartRouter())
	_, s, err := mgr.ExecuteSandboxQuery(context.Background(), "unknown-id", "hola")
	if err != nil || s.TenantID != "TENANT_DEMO_001" || s.QueryCount != 1 {
		t.Fatalf("auto-created session = %+v, err %v", s, err)
	}
}

// A session created by one instance is read by another with the same
// tenant_id and query_count.
func TestSPEC_CORE_73_R11_SessionIsSharedThroughRedis(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()

	blue := NewSessionManagerWithStore(router.NewDynamicSmartRouter(), NewRedisSessionStore(client))
	green := NewSessionManagerWithStore(router.NewDynamicSmartRouter(), NewRedisSessionStore(client))

	created, err := blue.CreateEphemeralSession(ctx, "f.pdf")
	if err != nil {
		t.Fatal(err)
	}
	key := sessionKeyPrefix + created.SessionID
	if ttl := mr.TTL(key); ttl <= 29*time.Minute || ttl > 30*time.Minute {
		t.Fatalf("ttl of %s = %s, want 30m", key, ttl)
	}
	if _, s, err := blue.ExecuteSandboxQuery(ctx, created.SessionID, "uno"); err != nil || s.QueryCount != 1 {
		t.Fatalf("blue: %+v %v", s, err)
	}
	_, s, err := green.ExecuteSandboxQuery(ctx, created.SessionID, "dos")
	if err != nil {
		t.Fatal(err)
	}
	if s.TenantID != created.TenantID || s.QueryCount != 2 {
		t.Fatalf("green saw %+v, want tenant %s and query_count 2", s, created.TenantID)
	}

	// An expired session is gone for both colors.
	mr.FastForward(31 * time.Minute)
	if _, ok, _ := NewRedisSessionStore(client).Get(ctx, created.SessionID); ok {
		t.Fatal("session should expire after 30 minutes")
	}
}

func TestSPEC_CORE_73_R11_RedisErrorIsReported(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	mr.Close()
	mgr := NewSessionManagerWithStore(router.NewDynamicSmartRouter(), NewRedisSessionStore(client))
	if _, err := mgr.CreateEphemeralSession(context.Background(), "f.pdf"); err == nil {
		t.Fatal("expected an error with Redis down")
	}
}
