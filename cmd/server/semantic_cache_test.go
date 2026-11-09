package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/martinllanos/only-ai-pods/internal/auth"
	"github.com/martinllanos/only-ai-pods/internal/pod"
	"github.com/martinllanos/only-ai-pods/internal/rag"
	"github.com/martinllanos/only-ai-pods/internal/store"
	"github.com/martinllanos/only-ai-pods/internal/vault"
)

func newFeedbackServer(t *testing.T) (*gin.Engine, *rag.SemanticCacheManager) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	v, err := vault.NewNativeVaultManager("0123456789abcdef0123456789abcdef", store.NewMemory())
	if err != nil {
		t.Fatal(err)
	}
	cache, err := rag.NewSemanticCacheManager(rag.NewMemoryCacheBackend(), v, time.Hour, 2*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.POST("/api/v1/feedback", feedbackHandler(&auth.Guard{}, cache))
	return r, cache
}

func postFeedback(t *testing.T, r *gin.Engine, body string) map[string]any {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/feedback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSPEC_CORE_71_R4_FeedbackEndpointPromotesAndPurges(t *testing.T) {
	r, cache := newFeedbackServer(t)
	ctx := context.Background()
	q := "¿Qué es el régimen de percepciones?"
	cache.StoreResponse(ctx, "tenant_a", q, &pod.PodResponse{PodID: "POD_AFIP_FISCAL", Answer: "x", Model: "m"}, false)

	up := postFeedback(t, r, `{"type":"up","tenant_id":"tenant_a","query":"¿Qué es el régimen de percepciones?"}`)
	if up["cache_promoted"] != true {
		t.Fatalf("👍 should promote, got %v", up)
	}
	if got, _ := cache.GetCachedResponse(ctx, "tenant_a", q); got == nil || got.CacheStatus != rag.CacheVerified {
		t.Fatalf("expected VERIFIED entry, got %+v", got)
	}

	down := postFeedback(t, r, `{"type":"down","tenant_id":"tenant_a","query":"¿Qué es el régimen de percepciones?","reason":"incorrecta"}`)
	if down["cache_purged"] != true {
		t.Fatalf("👎 should purge, got %v", down)
	}
	if _, ok := cache.GetCachedResponse(ctx, "tenant_a", q); ok {
		t.Fatal("after 👎 the next question must run the primary search")
	}
}

func TestSPEC_CORE_71_R4_FeedbackRejectsUnknownType(t *testing.T) {
	r, _ := newFeedbackServer(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/feedback", strings.NewReader(`{"type":"meh","query":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSPEC_CORE_71_R5_CacheTTLFromEnv(t *testing.T) {
	env := map[string]string{"CACHE_TTL_PROVISIONAL": "12h", "CACHE_TTL_VERIFIED": "nope"}
	get := func(k string) string { return env[k] }
	if d := cacheTTL(get, "CACHE_TTL_PROVISIONAL", rag.DefaultTTLProv); d != 12*time.Hour {
		t.Errorf("got %s", d)
	}
	if d := cacheTTL(get, "CACHE_TTL_VERIFIED", rag.DefaultTTLVerified); d != rag.DefaultTTLVerified {
		t.Errorf("invalid value should use the default, got %s", d)
	}
}
