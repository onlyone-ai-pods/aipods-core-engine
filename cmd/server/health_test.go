package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakePinger struct {
	err   error
	calls atomic.Int32
}

func (f *fakePinger) Ping(context.Context) error { f.calls.Add(1); return f.err }
func (f *fakePinger) Name() string               { return "postgres" }

func probeRouter(h *health) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerHealthRoutes(r, h)
	return r
}

func getJSON(t *testing.T, r http.Handler, path string) (int, map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("%s: invalid JSON %q: %v", path, w.Body.String(), err)
	}
	return w.Code, body
}

func TestSPEC_CORE_73_R5_ReadyzStates(t *testing.T) {
	db := &fakePinger{}
	h := newHealth(db, "41.0.0-abc1234", "green")
	r := probeRouter(h)

	code, body := getJSON(t, r, "/readyz")
	if code != http.StatusOK || body["status"] != "ready" || body["color"] != "green" || body["version"] != "41.0.0-abc1234" {
		t.Fatalf("ready: got %d %v", code, body)
	}

	db.err = errors.New("connection refused")
	if code, body = getJSON(t, r, "/readyz"); code != http.StatusServiceUnavailable || body["status"] != "not_ready" {
		t.Fatalf("database down: got %d %v", code, body)
	}

	db.err = nil
	h.StartDraining()
	if code, body = getJSON(t, r, "/readyz"); code != http.StatusServiceUnavailable || body["status"] != "draining" {
		t.Fatalf("draining: got %d %v", code, body)
	}
}

func TestSPEC_CORE_73_R5_LivezIgnoresDependencies(t *testing.T) {
	db := &fakePinger{err: errors.New("down")}
	h := newHealth(db, "v", "blue")
	h.StartDraining()
	code, body := getJSON(t, probeRouter(h), "/livez")
	if code != http.StatusOK || body["status"] != "alive" {
		t.Fatalf("livez: got %d %v", code, body)
	}
	if db.calls.Load() != 0 {
		t.Fatalf("livez pinged the database %d times", db.calls.Load())
	}
}

func TestSPEC_CORE_73_R6_HealthzDatabaseStatusIsCached(t *testing.T) {
	db := &fakePinger{}
	h := newHealth(db, "v", "")
	for range 5 {
		if got := h.databaseStatus(context.Background()); got != "postgres" {
			t.Fatalf("status = %q", got)
		}
	}
	if n := db.calls.Load(); n != 1 {
		t.Fatalf("5 /healthz calls pinged %d times, want 1", n)
	}

	// A readiness probe refreshes the cached state.
	db.err = errors.New("down")
	getJSON(t, probeRouter(h), "/readyz")
	if got := h.databaseStatus(context.Background()); got != "postgres:down" {
		t.Fatalf("after failed readiness, status = %q", got)
	}
}

func TestSPEC_CORE_73_R2_DrainMiddlewareClosesConnections(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newHealth(&fakePinger{}, "v", "")
	r := gin.New()
	r.Use(h.drainMiddleware())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Header().Get("Connection") != "" {
		t.Fatalf("Connection header set before draining")
	}
	h.StartDraining()
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Header().Get("Connection") != "close" {
		t.Fatalf("Connection = %q while draining, want close", w.Header().Get("Connection"))
	}
}
