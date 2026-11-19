package main

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/martinllanos/only-ai-pods/internal/router"
	"github.com/martinllanos/only-ai-pods/internal/store"
)

// defaultDynamicPodsSyncInterval is how often each instance rereads the
// dynamic pods registered by any instance (SPEC-CORE-73.R10).
const defaultDynamicPodsSyncInterval = 30 * time.Second

// dynamicPodStore is the part of store.Store the pod sync needs.
type dynamicPodStore interface {
	UpsertDynamicPod(ctx context.Context, p store.DynamicPod) error
	ListDynamicPods(ctx context.Context) ([]store.DynamicPod, error)
}

// podSyncer is the part of the router the pod sync needs.
type podSyncer interface {
	SyncDynamicPods(configs []router.DynamicPodConfig) int
}

func podToConfig(p store.DynamicPod) router.DynamicPodConfig {
	return router.DynamicPodConfig{PodID: p.PodID, Name: p.Name, TenantID: p.TenantID,
		EndpointURL: p.EndpointURL, Keywords: p.Keywords, Status: p.Status}
}

func configToPod(c router.DynamicPodConfig) store.DynamicPod {
	return store.DynamicPod{PodID: c.PodID, Name: c.Name, TenantID: c.TenantID,
		EndpointURL: c.EndpointURL, Keywords: c.Keywords, Status: c.Status}
}

// syncDynamicPods loads the stored pods into the router once.
func syncDynamicPods(ctx context.Context, st dynamicPodStore, r podSyncer) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	pods, err := st.ListDynamicPods(ctx)
	if err != nil {
		return 0, err
	}
	configs := make([]router.DynamicPodConfig, 0, len(pods))
	for _, p := range pods {
		configs = append(configs, podToConfig(p))
	}
	return r.SyncDynamicPods(configs), nil
}

// startDynamicPodSync rereads the pods every interval until ctx is cancelled
// (SIGTERM). The shutdown waits on wg before closing the database pool.
func startDynamicPodSync(ctx context.Context, wg *sync.WaitGroup, st dynamicPodStore, r podSyncer, interval time.Duration) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Println("shutdown: dynamic pod sync stopped")
				return
			case <-t.C:
				if n, err := syncDynamicPods(ctx, st, r); err != nil {
					if ctx.Err() == nil {
						log.Printf("dynamic pods: sync failed: %v", err)
					}
				} else if n > 0 {
					log.Printf("dynamic pods: %d registered from the store", n)
				}
			}
		}
	}()
}
