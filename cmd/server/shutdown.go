package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// Defaults for the drain sequence (SPEC-CORE-73.R2, R3). The container's
// stop_grace_period must be longer than DRAIN_DELAY + SHUTDOWN_TIMEOUT.
const (
	defaultDrainDelay      = 5 * time.Second
	defaultShutdownTimeout = 60 * time.Second
)

// durationEnv reads a Go duration ("5s", "1m") from the environment. Empty or
// invalid values fall back to the default, with a warning for invalid ones.
func durationEnv(getenv func(string) string, name string, def time.Duration) time.Duration {
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

// drainConfig holds the timings of the shutdown sequence.
type drainConfig struct {
	DrainDelay      time.Duration // readiness 503 before closing the listener
	ShutdownTimeout time.Duration // wait for in-flight requests
}

func loadDrainConfig(getenv func(string) string) drainConfig {
	return drainConfig{
		DrainDelay:      durationEnv(getenv, "DRAIN_DELAY", defaultDrainDelay),
		ShutdownTimeout: durationEnv(getenv, "SHUTDOWN_TIMEOUT", defaultShutdownTimeout),
	}
}

// newHTTPServer builds the server with timeouts (SPEC-CORE-73.R1). There is no
// global WriteTimeout: chat requests wait for the LLM, bounded by LLM_TIMEOUT.
// Every request context derives from base, so cancelling base aborts them.
func newHTTPServer(addr string, handler http.Handler, base context.Context) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		// Longer than Traefik's idleConnTimeout (90s), so the proxy closes
		// idle connections before the server does.
		IdleTimeout: 120 * time.Second,
		BaseContext: func(net.Listener) context.Context { return base },
	}
}

// serveUntilSignal serves on ln until ctx is cancelled (SIGINT/SIGTERM), then
// drains (SPEC-CORE-73.R2):
//
//  1. stopSignals restores the default handling, so a second signal kills;
//  2. readiness switches to 503;
//  3. it waits DrainDelay for the proxy to stop routing here;
//  4. srv.Shutdown stops accepting and waits for in-flight requests;
//  5. past ShutdownTimeout it cancels the request contexts and closes the server.
//
// The caller closes Redis and Postgres after it returns.
func serveUntilSignal(ctx context.Context, srv *http.Server, ln net.Listener, h *health,
	cfg drainConfig, stopSignals func(), cancelInflight context.CancelFunc) error {
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("http: %w", err)
	case <-ctx.Done():
	}
	stopSignals()

	log.Printf("shutdown: draining, readiness 503 for %s", cfg.DrainDelay)
	h.StartDraining()
	time.Sleep(cfg.DrainDelay)

	log.Printf("shutdown: waiting up to %s for in-flight requests", cfg.ShutdownTimeout)
	shCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shCtx); err != nil {
		log.Printf("shutdown: timeout reached, cancelling in-flight requests: %v", err)
		cancelInflight()
		if cerr := srv.Close(); cerr != nil {
			log.Printf("shutdown: close: %v", cerr)
		}
	}
	if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http: %w", err)
	}
	log.Println("shutdown: HTTP server stopped")
	return nil
}
