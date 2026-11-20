package main

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/martinllanos/only-ai-pods/internal/auth"
	"github.com/martinllanos/only-ai-pods/internal/llm"
	"github.com/martinllanos/only-ai-pods/internal/store"
)

func settings(t *testing.T, env map[string]string) llmSettings {
	t.Helper()
	s, err := loadLLMSettings(envMap(env), "claude-opus-5-5", 45*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSPEC_CORE_74_R1_WithoutNewVariablesEveryTaskUsesAnthropicModel(t *testing.T) {
	s := settings(t, nil)
	want := llm.Models{Classify: "claude-opus-5-5", Expand: "claude-opus-5-5", Answer: "claude-opus-5-5"}
	if !reflect.DeepEqual(s.defaults, want) {
		t.Fatalf("defaults = %+v", s.defaults)
	}
	for _, task := range llm.Tasks {
		if s.budget[task] != 45*time.Second {
			t.Errorf("%s budget = %s, want LLM_TIMEOUT", task, s.budget[task])
		}
	}
	if s.attemptTimeout != 0 {
		t.Errorf("attempt timeout = %s, want 0 (half the budget)", s.attemptTimeout)
	}
}

func TestSPEC_CORE_74_R1_OneVariableChangesOneTask(t *testing.T) {
	s := settings(t, map[string]string{"LLM_MODEL_CLASSIFY": " claude-haiku-5-5 ", "LLM_MODEL_EXPAND": "   "})
	want := llm.Models{Classify: "claude-haiku-5-5", Expand: "claude-opus-5-5", Answer: "claude-opus-5-5"}
	if !reflect.DeepEqual(s.defaults, want) {
		t.Fatalf("defaults = %+v", s.defaults)
	}
}

func TestSPEC_CORE_74_R2_ModelOutsideTheAllowedListStopsTheStart(t *testing.T) {
	for name, env := range map[string]map[string]string{
		"LLM_MODEL_ANSWER":    {"LLM_MODEL_ANSWER": "claude-opus-9"},
		"LLM_FALLBACK_MODELS": {"LLM_FALLBACK_MODELS": "claude-sonnet-5-5,gpt-x"},
	} {
		_, err := buildAIStack(envMap(env))
		if err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("%s: err = %v, want an error naming the variable", name, err)
		}
	}
	// ANTHROPIC_MODEL is always allowed, and LLM_ALLOWED_MODELS extends the list.
	if _, err := buildAIStack(envMap(map[string]string{"ANTHROPIC_MODEL": "claude-opus-5"})); err != nil {
		t.Errorf("ANTHROPIC_MODEL must always be allowed: %v", err)
	}
	s, err := buildAIStack(envMap(map[string]string{
		"LLM_ALLOWED_MODELS": "claude-opus-5-5,claude-fable-5-2", "LLM_MODEL_ANSWER": "claude-fable-5-2", "ANTHROPIC_API_KEY": "k",
	}))
	if err != nil || s.models.defaults.Answer != "claude-fable-5-2" || s.llmMode != "anthropic:claude-fable-5-2" {
		t.Errorf("custom allowed list: %+v, mode %q, %v", s.models.defaults, s.llmMode, err)
	}
}

func TestSPEC_CORE_74_R10_FallbackChainLimits(t *testing.T) {
	s := settings(t, map[string]string{"LLM_FALLBACK_MODELS": "claude-sonnet-5-5, claude-haiku-5-5"})
	if !reflect.DeepEqual(s.defaults.Fallback, []string{"claude-sonnet-5-5", "claude-haiku-5-5"}) {
		t.Fatalf("fallback = %v", s.defaults.Fallback)
	}
	_, err := loadLLMSettings(envMap(map[string]string{
		"LLM_ALLOWED_MODELS":  "a,b,c,d",
		"LLM_FALLBACK_MODELS": "a,b,c,d",
	}), "claude-opus-5-5", time.Minute)
	if err == nil {
		t.Fatal("4 fallback models must stop the start")
	}
}

func TestSPEC_CORE_74_R11_TaskTimeouts(t *testing.T) {
	s := settings(t, map[string]string{
		"LLM_TIMEOUT_CLASSIFY": "10s",
		"LLM_TIMEOUT_EXPAND":   "2m",   // longer than LLM_TIMEOUT: capped
		"LLM_TIMEOUT_ANSWER":   "nope", // invalid: default
		"LLM_ATTEMPT_TIMEOUT":  "7s",
	})
	if s.budget[llm.TaskClassify] != 10*time.Second || s.budget[llm.TaskExpand] != 45*time.Second ||
		s.budget[llm.TaskAnswer] != 45*time.Second || s.attemptTimeout != 7*time.Second {
		t.Fatalf("budget = %v attempt = %s", s.budget, s.attemptTimeout)
	}
}

func TestSPEC_CORE_74_R6_Precedence(t *testing.T) {
	s := settings(t, map[string]string{"LLM_MODEL_CLASSIFY": "claude-haiku-5-5", "LLM_FALLBACK_MODELS": "claude-sonnet-5-5"})
	st := store.NewMemory()
	ctx := context.Background()
	for _, id := range []string{"inherits", "own", "stale", "nochain"} {
		if err := st.EnsureTenant(ctx, id, id); err != nil {
			t.Fatal(err)
		}
	}
	_ = st.SetLLMModels(ctx, "own", store.TenantLLMModels{Answer: "claude-sonnet-5-5", Fallback: []string{"claude-haiku-5-5"}, FallbackSet: true})
	_ = st.SetLLMModels(ctx, "stale", store.TenantLLMModels{Answer: "claude-retired-1", Fallback: []string{"claude-retired-1", "claude-haiku-5-5"}, FallbackSet: true})
	_ = st.SetLLMModels(ctx, "nochain", store.TenantLLMModels{FallbackSet: true})

	resolve := storeModelResolver(st, s)
	platform := llm.Models{Classify: "claude-haiku-5-5", Expand: "claude-opus-5-5", Answer: "claude-opus-5-5", Fallback: []string{"claude-sonnet-5-5"}}
	cases := map[string]llm.Models{
		"inherits":        platform,
		"TENANT_DEMO_001": platform, // no row
		"own":             {Classify: "claude-haiku-5-5", Expand: "claude-opus-5-5", Answer: "claude-sonnet-5-5", Fallback: []string{"claude-haiku-5-5"}},
		"stale":           {Classify: "claude-haiku-5-5", Expand: "claude-opus-5-5", Answer: "claude-opus-5-5", Fallback: []string{"claude-haiku-5-5"}},
		"nochain":         {Classify: "claude-haiku-5-5", Expand: "claude-opus-5-5", Answer: "claude-opus-5-5", Fallback: []string{}},
	}
	for id, want := range cases {
		if got := resolve(ctx, id); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %+v, want %+v", id, got, want)
		}
	}
}

// R8: only a platform_admin reads and changes a tenant's LLM models; the
// change is validated against the allowed list and audited.
func TestSPEC_CORE_74_R8_LLMModelsAPI(t *testing.T) {
	api := newUserAPI(t)
	s := settings(t, map[string]string{"LLM_MODEL_CLASSIFY": "claude-haiku-5-5"})
	registerLLMModelRoutes(api.r, api.st, api.guard, auditor{st: api.st, guard: api.guard}, s)

	root := api.login("root@aipods.ar", "root-password-123")
	api.create(root, "tenant_acme", "jefa@acme.test", auth.RoleTenantAdmin)
	jefa := api.login("jefa@acme.test", "initial-pass-123")
	const path = "/api/v1/admin/tenants/tenant_acme/llm-models"

	code, out := api.call("GET", path, root, nil)
	if code != 200 {
		t.Fatalf("GET: %d %v", code, out)
	}
	configured := out["configured"].(map[string]any)
	effective := out["effective"].(map[string]any)
	if configured["answer"] != nil || configured["fallback"] != nil || effective["classify"] != "claude-haiku-5-5" ||
		effective["answer"] != "claude-opus-5-5" || len(out["allowed"].([]any)) != 3 {
		t.Fatalf("default view: %v", out)
	}

	if code, _ := api.call("GET", path, jefa, nil); code != 403 {
		t.Errorf("tenant_admin GET: want 403, got %d", code)
	}
	if code, _ := api.call("PUT", path, jefa, map[string]any{"answer": "claude-sonnet-5-5"}); code != 403 {
		t.Errorf("tenant_admin PUT: want 403, got %d", code)
	}
	if code, out := api.call("PUT", path, root, map[string]any{"answer": "gpt-x"}); code != 400 || out["allowed"] == nil {
		t.Errorf("model not allowed: want 400 with the allowed list, got %d %v", code, out)
	}
	if code, _ := api.call("PUT", path, root, map[string]any{"fallback": []string{"claude-sonnet-5-5", "nope"}}); code != 400 {
		t.Errorf("fallback not allowed: want 400, got %d", code)
	}
	if code, _ := api.call("PUT", "/api/v1/admin/tenants/NOPE/llm-models", root, map[string]any{"answer": "claude-sonnet-5-5"}); code != 404 {
		t.Errorf("unknown tenant: want 404, got %d", code)
	}
	if code, _ := api.call("GET", "/api/v1/admin/tenants/NOPE/llm-models", root, nil); code != 404 {
		t.Errorf("unknown tenant GET: want 404, got %d", code)
	}

	code, out = api.call("PUT", path, root, map[string]any{
		"answer": "claude-sonnet-5-5", "expand": "", "fallback": []string{"claude-haiku-5-5"},
	})
	if code != 200 {
		t.Fatalf("PUT: %d %v", code, out)
	}
	configured, effective = out["configured"].(map[string]any), out["effective"].(map[string]any)
	if configured["answer"] != "claude-sonnet-5-5" || configured["expand"] != nil || configured["classify"] != nil ||
		effective["answer"] != "claude-sonnet-5-5" || effective["classify"] != "claude-haiku-5-5" ||
		!reflect.DeepEqual(effective["fallback"], []any{"claude-haiku-5-5"}) {
		t.Fatalf("after PUT: %v", out)
	}

	// null everywhere inherits again.
	if code, out := api.call("PUT", path, root, map[string]any{}); code != 200 || out["configured"].(map[string]any)["answer"] != nil {
		t.Fatalf("inherit: %d %v", code, out)
	}

	events, err := api.st.ListAudit(context.Background(), "tenant_acme", 50)
	if err != nil {
		t.Fatal(err)
	}
	var ok, rejected bool
	for _, e := range events {
		if e.Action != "llm_models.set" {
			continue
		}
		ok = ok || (e.Outcome == "success" && strings.Contains(e.Target, "answer=claude-sonnet-5-5") && strings.Contains(e.Target, "fallback=[claude-haiku-5-5]"))
		rejected = rejected || e.Outcome == "error"
	}
	if !ok || !rejected {
		t.Errorf("audit must hold the accepted and the rejected change: ok=%t rejected=%t %+v", ok, rejected, events)
	}
}

func TestSPEC_CORE_74_R10_APIRejectsLongChains(t *testing.T) {
	s, err := loadLLMSettings(envMap(map[string]string{"LLM_ALLOWED_MODELS": "a,b,c,d"}), "a", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	four := []string{"a", "b", "c", "d"}
	if _, err := s.parseRequest(SetLLMModelsRequest{Fallback: &four}); err == nil {
		t.Fatal("4 fallback models must be rejected")
	}
	none := []string{}
	got, err := s.parseRequest(SetLLMModelsRequest{Fallback: &none})
	if err != nil || !got.FallbackSet || len(got.Fallback) != 0 {
		t.Fatalf("an empty list means no chain: %+v %v", got, err)
	}
}

func TestSPEC_CORE_74_R16_HealthzShowsTheModels(t *testing.T) {
	ai, err := buildAIStack(envMap(map[string]string{
		"ANTHROPIC_API_KEY": "k", "LLM_MODEL_CLASSIFY": "claude-haiku-5-5", "LLM_FALLBACK_MODELS": "claude-sonnet-5-5",
	}))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(healthzPayload("v", "blue", ai, "postgres", "postgres:up", "enforce"))
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	if body["llm"] != "anthropic:claude-opus-5-5" {
		t.Errorf("llm = %v, want the previous format", body["llm"])
	}
	want := map[string]any{"classify": "claude-haiku-5-5", "expand": "claude-opus-5-5", "answer": "claude-opus-5-5", "fallback": []any{"claude-sonnet-5-5"}}
	if !reflect.DeepEqual(body["llm_models"], want) {
		t.Errorf("llm_models = %v", body["llm_models"])
	}
}
