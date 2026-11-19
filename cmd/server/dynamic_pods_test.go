package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/martinllanos/only-ai-pods/internal/router"
	"github.com/martinllanos/only-ai-pods/internal/store"
)

// A pod registered through instance A shows up in instance B after a sync.
func TestSPEC_CORE_73_R10_PodRegisteredOnOneInstanceReachesTheOther(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	a, b := router.NewDynamicSmartRouter(), router.NewDynamicSmartRouter()
	base := b.PodCount()

	cfg := router.DynamicPodConfig{PodID: "POD_X", Name: "X", TenantID: "t1", EndpointURL: "http://pod-x:9000", Keywords: []string{"x"}}
	if err := st.UpsertDynamicPod(ctx, configToPod(cfg)); err != nil {
		t.Fatal(err)
	}
	a.RegisterDynamicPod(cfg)

	n, err := syncDynamicPods(ctx, st, b)
	if err != nil || n != 1 || b.PodCount() != base+1 {
		t.Fatalf("sync: n=%d err=%v pods=%d", n, err, b.PodCount())
	}
	if n, _ := syncDynamicPods(ctx, st, b); n != 0 {
		t.Fatalf("an unchanged pod must not be registered again, got %d", n)
	}
	cfg.EndpointURL = "http://pod-x:9001"
	_ = st.UpsertDynamicPod(ctx, configToPod(cfg))
	if n, _ := syncDynamicPods(ctx, st, b); n != 1 {
		t.Fatalf("a changed pod must be registered again, got %d", n)
	}
}

type countingSyncer struct {
	mu    sync.Mutex
	calls int
}

func (c *countingSyncer) SyncDynamicPods([]router.DynamicPodConfig) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return 0
}

func (c *countingSyncer) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func TestSPEC_CORE_73_R10_SyncWorkerStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	syncer := &countingSyncer{}
	startDynamicPodSync(ctx, &wg, store.NewMemory(), syncer, 10*time.Millisecond)

	deadline := time.Now().Add(2 * time.Second)
	for syncer.count() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if syncer.count() < 2 {
		t.Fatal("worker did not tick")
	}
	cancel()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after cancel")
	}
}
