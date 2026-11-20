package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/martinllanos/only-ai-pods/internal/auth"
	"github.com/martinllanos/only-ai-pods/internal/router"
	"github.com/martinllanos/only-ai-pods/internal/store"
	"github.com/martinllanos/only-ai-pods/internal/telemetry"
)

func chatServer(t *testing.T) (*gin.Engine, store.Store, *telemetry.MetricsCollector) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	st := store.NewMemory()
	m := telemetry.NewMetricsCollector()
	sr := router.NewDynamicSmartRouter()
	sr.SetIntentHook(m.RecordPodIntent)
	r := gin.New()
	registerChatRoute(r, &auth.Guard{Mode: auth.ModeOff}, sr, st, m)
	return r, st, m
}

func postChat(t *testing.T, r *gin.Engine, tenantID, message string) map[string]any {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"tenant_id": tenantID, "message": message})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/chat/completions", strings.NewReader(string(body))))
	if w.Code != 200 {
		t.Fatalf("chat %q: status %d %s", message, w.Code, w.Body.String())
	}
	out := map[string]any{}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// SPEC-CORE-72.R2: an informational chat creates no approval rows.
// SPEC-CORE-72.R3: an operational one still creates its approval.
func TestSPEC_CORE_72_R2_InformationalChatCreatesNoApproval(t *testing.T) {
	r, st, m := chatServer(t)
	ctx := context.Background()

	for _, q := range []string{
		"¿Cuáles son las categorías del Monotributo vigentes?",
		"¿Cómo doy de alta un punto de venta?",
		"¿Qué es la clave fiscal?",
	} {
		out := postChat(t, r, "tenant_acme", q)
		if _, ok := out["dry_run_result"]; ok {
			t.Errorf("%q: informational answer must not carry dry_run_result: %v", q, out["dry_run_result"])
		}
		if _, ok := out["intent"]; ok {
			t.Error("intent is internal and must not change the API contract")
		}
	}
	if items, err := st.ListApprovals(ctx, ""); err != nil || len(items) != 0 {
		t.Fatalf("informational chats created %d approvals (err %v)", len(items), err)
	}

	// Without an LLM the keyword router sends "estado de cuenta" to billing,
	// so use an operational query that reaches the AFIP pod.
	postChat(t, r, "tenant_acme", "Descargá mis comprobantes recibidos de octubre")
	items, err := st.ListApprovals(ctx, "")
	if err != nil || len(items) != 1 || items[0].Status != "PENDING" || items[0].TenantID != "tenant_acme" {
		t.Fatalf("operational chat must create one pending approval, got %+v (err %v)", items, err)
	}

	// SPEC-CORE-72.R4 end to end: 3 informational + 1 operational.
	s := m.Snapshot()
	if s.AFIPByIntent["informational"] != 3 || s.AFIPByIntent["operational"] != 1 {
		t.Errorf("intent counters = %v", s.AFIPByIntent)
	}
}

// SPEC-CORE-72.R6: RAG_TOP_K configuration.
func TestSPEC_CORE_72_R6_RAGTopKFromEnv(t *testing.T) {
	for _, tc := range []struct {
		env  string
		want int
	}{
		{"", 10}, {"8", 8}, {"50", 10}, {"abc", 10},
	} {
		got := ragTopKFromEnv(func(k string) string {
			if k == "RAG_TOP_K" {
				return tc.env
			}
			return ""
		})
		if got != tc.want {
			t.Errorf("RAG_TOP_K=%q: got %d, want %d", tc.env, got, tc.want)
		}
	}
}
