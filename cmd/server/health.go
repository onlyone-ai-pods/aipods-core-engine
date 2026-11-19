package main

import (
	"context"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// buildVersion is injected at build time:
//
//	go build -ldflags "-X main.buildVersion=<VERSION>-<short sha>"
//
// It identifies which deployment (blue or green) answered (SPEC-CORE-73.R6).
var buildVersion = "dev"

// healthDBCacheTTL bounds how often /healthz pings the database: the endpoint
// is public, so it must not turn every call into a query (SPEC-CORE-73.R6).
const healthDBCacheTTL = 10 * time.Second

// pinger is the part of the store the probes need.
type pinger interface {
	Ping(ctx context.Context) error
	Name() string
}

// health owns the liveness and readiness probes and the draining flag
// (SPEC-CORE-73.R2, R5, R6).
type health struct {
	draining atomic.Bool
	db       pinger
	version  string
	color    string

	mu        sync.Mutex
	lastCheck time.Time
	lastErr   error
	checked   bool
}

func newHealth(db pinger, version, color string) *health {
	return &health{db: db, version: version, color: color}
}

// StartDraining makes /readyz answer 503 so the proxy stops sending new traffic.
func (h *health) StartDraining() { h.draining.Store(true) }

// Draining reports whether shutdown has started.
func (h *health) Draining() bool { return h.draining.Load() }

// ping checks the database and remembers the result for /healthz.
func (h *health) ping(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	err := h.db.Ping(ctx)
	h.mu.Lock()
	h.lastCheck, h.lastErr, h.checked = time.Now(), err, true
	h.mu.Unlock()
	return err
}

// databaseStatus returns the database state for /healthz, pinging at most once
// per healthDBCacheTTL.
func (h *health) databaseStatus(ctx context.Context) string {
	h.mu.Lock()
	fresh := h.checked && time.Since(h.lastCheck) < healthDBCacheTTL
	err := h.lastErr
	h.mu.Unlock()
	if !fresh {
		err = h.ping(ctx, 2*time.Second)
	}
	if err != nil {
		return h.db.Name() + ":down"
	}
	return h.db.Name()
}

// live answers 200 while the process serves requests. It never checks
// dependencies: a database outage must not make Docker restart the core.
func (h *health) live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "alive", "version": h.version, "color": h.color})
}

// ready answers 503 while draining or when the database is unreachable, and
// 200 when this instance can take new traffic.
func (h *health) ready(c *gin.Context) {
	if h.Draining() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "draining", "version": h.version, "color": h.color})
		return
	}
	if err := h.ping(c.Request.Context(), time.Second); err != nil {
		log.Printf("readyz: database not ready: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "database": "down", "version": h.version, "color": h.color})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready", "version": h.version, "color": h.color})
}

// drainMiddleware asks the proxy not to reuse the connection once draining
// has started, so its next request opens a connection to the other color.
func (h *health) drainMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Draining() {
			c.Header("Connection", "close")
		}
		c.Next()
	}
}

// registerHealthRoutes adds /livez and /readyz. Call it before the auth and
// rate-limit middleware: gin applies only the middleware registered earlier.
func registerHealthRoutes(r *gin.Engine, h *health) {
	r.GET("/livez", h.live)
	r.GET("/readyz", h.ready)
}
