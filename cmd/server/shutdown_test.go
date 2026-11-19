package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type testServer struct {
	url      string
	h        *health
	cancel   context.CancelFunc
	done     chan error
	inflight context.CancelFunc
}

func startServer(t *testing.T, r *gin.Engine, h *health, cfg drainConfig) *testServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	base, cancelInflight := context.WithCancel(context.Background())
	srv := newHTTPServer(ln.Addr().String(), r, base)
	ts := &testServer{url: "http://" + ln.Addr().String(), h: h, cancel: cancel, done: make(chan error, 1), inflight: cancelInflight}
	go func() { ts.done <- serveUntilSignal(ctx, srv, ln, h, cfg, func() {}, cancelInflight) }()
	t.Cleanup(func() { cancel(); cancelInflight() })
	return ts
}

func TestSPEC_CORE_73_R2_InFlightRequestFinishesDuringDrain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newHealth(&fakePinger{}, "v", "blue")
	started := make(chan struct{})
	r := gin.New()
	registerHealthRoutes(r, h)
	r.GET("/slow", func(c *gin.Context) {
		close(started)
		time.Sleep(600 * time.Millisecond)
		c.String(http.StatusOK, "done")
	})
	ts := startServer(t, r, h, drainConfig{DrainDelay: 200 * time.Millisecond, ShutdownTimeout: 5 * time.Second})

	type result struct {
		code int
		body string
		err  error
	}
	slow := make(chan result, 1)
	go func() {
		resp, err := http.Get(ts.url + "/slow")
		if err != nil {
			slow <- result{err: err}
			return
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		slow <- result{code: resp.StatusCode, body: string(b)}
	}()
	<-started
	ts.cancel() // SIGTERM

	// During the drain delay readiness is already 503 and the server still answers.
	time.Sleep(50 * time.Millisecond)
	resp, err := http.Get(ts.url + "/readyz")
	if err != nil {
		t.Fatalf("readyz during drain: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("readyz during drain = %d, want 503", resp.StatusCode)
	}

	res := <-slow
	if res.err != nil || res.code != http.StatusOK || res.body != "done" {
		t.Fatalf("in-flight request: %+v", res)
	}
	if err := <-ts.done; err != nil {
		t.Fatalf("serveUntilSignal: %v", err)
	}
	if _, err := http.Get(ts.url + "/livez"); err == nil {
		t.Fatalf("server still accepts connections after shutdown")
	}
}

func TestSPEC_CORE_73_R2_TimeoutCancelsInFlightRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newHealth(&fakePinger{}, "v", "")
	started := make(chan struct{})
	cancelled := make(chan struct{})
	r := gin.New()
	r.GET("/stuck", func(c *gin.Context) {
		close(started)
		<-c.Request.Context().Done() // like an LLM call that honours ctx
		close(cancelled)
	})
	ts := startServer(t, r, h, drainConfig{DrainDelay: 10 * time.Millisecond, ShutdownTimeout: 200 * time.Millisecond})

	go func() {
		if resp, err := http.Get(ts.url + "/stuck"); err == nil {
			resp.Body.Close()
		}
	}()
	<-started
	begin := time.Now()
	ts.cancel()

	select {
	case <-cancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("in-flight request context was not cancelled after SHUTDOWN_TIMEOUT")
	}
	if err := <-ts.done; err != nil {
		t.Fatalf("serveUntilSignal: %v", err)
	}
	if elapsed := time.Since(begin); elapsed > 2*time.Second {
		t.Fatalf("shutdown took %s", elapsed)
	}
}

func TestSPEC_CORE_73_R1_RunStopsOnSignal(t *testing.T) {
	env := map[string]string{"PORT": "0", "DRAIN_DELAY": "10ms", "SHUTDOWN_TIMEOUT": "1s", "DEPLOY_COLOR": "green"}
	getenv := func(k string) string { return env[k] }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, func() {}, getenv) }()
	time.Sleep(300 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run did not stop after the signal")
	}
}

func TestSPEC_CORE_73_R1_RunReturnsInitErrors(t *testing.T) {
	env := map[string]string{"PORT": "0", "AUTH_MODE": "bogus"}
	err := run(context.Background(), func() {}, func(k string) string { return env[k] })
	if err == nil || !strings.Contains(err.Error(), "AUTH_MODE") {
		t.Fatalf("run = %v, want AUTH_MODE error", err)
	}
}

func TestSPEC_CORE_73_R2_DurationEnv(t *testing.T) {
	env := map[string]string{"A": "250ms", "B": "nope", "C": "-1s"}
	get := func(k string) string { return env[k] }
	if d := durationEnv(get, "A", time.Second); d != 250*time.Millisecond {
		t.Fatalf("A = %s", d)
	}
	for _, k := range []string{"B", "C", "MISSING"} {
		if d := durationEnv(get, k, time.Second); d != time.Second {
			t.Fatalf("%s = %s, want default", k, d)
		}
	}
	cfg := loadDrainConfig(func(string) string { return "" })
	if cfg.DrainDelay != defaultDrainDelay || cfg.ShutdownTimeout != defaultShutdownTimeout {
		t.Fatalf("defaults = %+v", cfg)
	}
}
