package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/martinllanos/only-ai-pods/internal/auth"
	"github.com/martinllanos/only-ai-pods/internal/llm"
	"github.com/martinllanos/only-ai-pods/internal/router"
	"github.com/martinllanos/only-ai-pods/internal/store"
)

// Model per task and per tenant, with a fallback chain (SPEC-CORE-74).

// defaultAllowedModels is LLM_ALLOWED_MODELS when it is not set (R2).
const defaultAllowedModels = "claude-opus-5-5,claude-sonnet-5-5,claude-haiku-5-5"

// taskModelEnv and taskTimeoutEnv name the variables of each task (R1, R11).
var (
	taskModelEnv = map[llm.Task]string{
		llm.TaskClassify: "LLM_MODEL_CLASSIFY",
		llm.TaskExpand:   "LLM_MODEL_EXPAND",
		llm.TaskAnswer:   "LLM_MODEL_ANSWER",
	}
	taskTimeoutEnv = map[llm.Task]string{
		llm.TaskClassify: "LLM_TIMEOUT_CLASSIFY",
		llm.TaskExpand:   "LLM_TIMEOUT_EXPAND",
		llm.TaskAnswer:   "LLM_TIMEOUT_ANSWER",
	}
)

// llmSettings is the platform-level LLM configuration read from the environment.
type llmSettings struct {
	// defaults always has a model for every task: the task's variable or
	// ANTHROPIC_MODEL.
	defaults       llm.Models
	allowed        []string
	budget         map[llm.Task]time.Duration
	attemptTimeout time.Duration
}

func (s llmSettings) isAllowed(model string) bool {
	for _, a := range s.allowed {
		if a == model {
			return true
		}
	}
	return false
}

// loadLLMSettings reads LLM_MODEL_*, LLM_FALLBACK_MODELS, LLM_ALLOWED_MODELS
// and the task timeouts. A model outside the allowed list is an error that
// names the variable, so the core does not start with a typo (R2).
func loadLLMSettings(getenv func(string) string, baseModel string, llmTimeout time.Duration) (llmSettings, error) {
	raw := getenv("LLM_ALLOWED_MODELS")
	if strings.TrimSpace(raw) == "" {
		raw = defaultAllowedModels
	}
	s := llmSettings{allowed: llm.ParseModelList(raw), budget: map[llm.Task]time.Duration{}}
	if !s.isAllowed(baseModel) { // ANTHROPIC_MODEL is always allowed
		s.allowed = append(s.allowed, baseModel)
	}

	pick := func(task llm.Task) (string, error) {
		name := taskModelEnv[task]
		v := strings.TrimSpace(getenv(name))
		if v == "" {
			return baseModel, nil
		}
		if !s.isAllowed(v) {
			return "", fmt.Errorf("%s=%q is not in LLM_ALLOWED_MODELS (%s)", name, v, strings.Join(s.allowed, ", "))
		}
		return v, nil
	}
	var err error
	if s.defaults.Classify, err = pick(llm.TaskClassify); err != nil {
		return llmSettings{}, err
	}
	if s.defaults.Expand, err = pick(llm.TaskExpand); err != nil {
		return llmSettings{}, err
	}
	if s.defaults.Answer, err = pick(llm.TaskAnswer); err != nil {
		return llmSettings{}, err
	}

	s.defaults.Fallback = llm.ParseModelList(getenv("LLM_FALLBACK_MODELS"))
	if err := s.validateChain(s.defaults.Fallback); err != nil {
		return llmSettings{}, fmt.Errorf("LLM_FALLBACK_MODELS: %w", err)
	}

	for _, task := range llm.Tasks {
		d := durationEnv(getenv, taskTimeoutEnv[task], llmTimeout)
		if d > llmTimeout {
			log.Printf("WARNING: %s (%s) is longer than LLM_TIMEOUT (%s); using LLM_TIMEOUT", taskTimeoutEnv[task], d, llmTimeout)
			d = llmTimeout
		}
		s.budget[task] = d
	}
	// Zero means half the task's budget (R11).
	if strings.TrimSpace(getenv("LLM_ATTEMPT_TIMEOUT")) != "" {
		s.attemptTimeout = durationEnv(getenv, "LLM_ATTEMPT_TIMEOUT", 0)
	}
	return s, nil
}

// validateChain checks a fallback chain: at most MaxFallbackModels, all allowed (R10).
func (s llmSettings) validateChain(chain []string) error {
	if len(chain) > llm.MaxFallbackModels {
		return fmt.Errorf("at most %d fallback models, got %d", llm.MaxFallbackModels, len(chain))
	}
	for _, m := range chain {
		if !s.isAllowed(m) {
			return fmt.Errorf("%q is not in LLM_ALLOWED_MODELS (%s)", m, strings.Join(s.allowed, ", "))
		}
	}
	return nil
}

// effective applies the tenant's own models over the platform's (R6). A
// stored model that is no longer allowed is ignored and reported.
func (s llmSettings) effective(t store.TenantLLMModels) (llm.Models, []string) {
	out := s.defaults
	out.Fallback = append([]string(nil), s.defaults.Fallback...)
	var ignored []string
	override := func(dst *string, v string) {
		switch {
		case v == "":
		case s.isAllowed(v):
			*dst = v
		default:
			ignored = append(ignored, v)
		}
	}
	override(&out.Classify, t.Classify)
	override(&out.Expand, t.Expand)
	override(&out.Answer, t.Answer)
	if t.FallbackSet {
		chain := []string{}
		for _, m := range t.Fallback {
			if s.isAllowed(m) {
				chain = append(chain, m)
			} else {
				ignored = append(ignored, m)
			}
		}
		out.Fallback = chain
	}
	return out, ignored
}

// storeModelResolver reads the tenant's models once per query (R7). Tenants
// without a row (AUTH_MODE=off demo tenants) or a failing lookup use the
// platform's configuration.
func storeModelResolver(st store.Store, s llmSettings) router.ModelResolver {
	return func(ctx context.Context, tenantID string) llm.Models {
		t, err := st.GetLLMModels(ctx, tenantID)
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				log.Printf("[llm] could not read the LLM models of tenant %s, using the platform's: %v", tenantID, err)
			}
			return s.defaults
		}
		models, ignored := s.effective(t)
		if len(ignored) > 0 {
			log.Printf("WARNING: [llm] tenant %s has models that are no longer allowed (%s); using the platform's", tenantID, strings.Join(ignored, ", "))
		}
		return models
	}
}

// SetLLMModelsRequest is the body of PUT .../llm-models. A null or empty task
// inherits the platform's model; a null fallback inherits the platform's
// chain and an empty list means no chain for the tenant (R8).
type SetLLMModelsRequest struct {
	Classify *string   `json:"classify"`
	Expand   *string   `json:"expand"`
	Answer   *string   `json:"answer"`
	Fallback *[]string `json:"fallback"`
}

func (s llmSettings) parseRequest(req SetLLMModelsRequest) (store.TenantLLMModels, error) {
	var out store.TenantLLMModels
	set := func(dst *string, field string, v *string) error {
		if v == nil {
			return nil
		}
		name := strings.TrimSpace(*v)
		if name != "" && !s.isAllowed(name) {
			return fmt.Errorf("%s: model %q is not allowed", field, name)
		}
		*dst = name
		return nil
	}
	if err := set(&out.Classify, "classify", req.Classify); err != nil {
		return out, err
	}
	if err := set(&out.Expand, "expand", req.Expand); err != nil {
		return out, err
	}
	if err := set(&out.Answer, "answer", req.Answer); err != nil {
		return out, err
	}
	if req.Fallback != nil {
		out.FallbackSet = true
		out.Fallback = llm.ParseModelList(strings.Join(*req.Fallback, ","))
		if err := s.validateChain(out.Fallback); err != nil {
			return out, fmt.Errorf("fallback: %w", err)
		}
	}
	return out, nil
}

// llmModelsView is the JSON shape of a model set: null means inherit.
func configuredView(t store.TenantLLMModels) gin.H {
	opt := func(v string) any {
		if v == "" {
			return nil
		}
		return v
	}
	var fallback any
	if t.FallbackSet {
		fallback = append([]string{}, t.Fallback...)
	}
	return gin.H{"classify": opt(t.Classify), "expand": opt(t.Expand), "answer": opt(t.Answer), "fallback": fallback}
}

func modelsView(m llm.Models) gin.H {
	return gin.H{"classify": m.Classify, "expand": m.Expand, "answer": m.Answer, "fallback": append([]string{}, m.Fallback...)}
}

// auditTarget is the audited summary of a change; it never holds secrets.
func auditTarget(t store.TenantLLMModels) string {
	or := func(v string) string {
		if v == "" {
			return "inherit"
		}
		return v
	}
	fallback := "inherit"
	if t.FallbackSet {
		fallback = "[" + strings.Join(t.Fallback, ",") + "]"
	}
	return fmt.Sprintf("classify=%s expand=%s answer=%s fallback=%s", or(t.Classify), or(t.Expand), or(t.Answer), fallback)
}

// registerLLMModelRoutes lets a platform_admin read and change a tenant's
// LLM models. Every change is audited, also when it is rejected (R8).
func registerLLMModelRoutes(r *gin.Engine, st store.Store, guard *auth.Guard, audit auditor, s llmSettings) {
	admin := guard.RequireRoles(auth.RolePlatformAdmin)
	respond := func(c *gin.Context, id string, t store.TenantLLMModels) {
		effective, _ := s.effective(t)
		c.JSON(http.StatusOK, gin.H{
			"tenant_id":  id,
			"configured": configuredView(t),
			"effective":  modelsView(effective),
			"platform":   modelsView(s.defaults),
			"allowed":    s.allowed,
		})
	}

	r.GET("/api/v1/admin/tenants/:id/llm-models", admin, func(c *gin.Context) {
		id := c.Param("id")
		t, err := st.GetLLMModels(c.Request.Context(), id)
		switch {
		case errors.Is(err, store.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		case err != nil:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read LLM models"})
		default:
			respond(c, id, t)
		}
	})

	r.PUT("/api/v1/admin/tenants/:id/llm-models", admin, func(c *gin.Context) {
		id := c.Param("id")
		var req SetLLMModelsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		t, err := s.parseRequest(req)
		if err != nil {
			audit.record(c, id, "llm_models.set", auditTarget(t), outcome(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "allowed": s.allowed})
			return
		}
		err = st.SetLLMModels(c.Request.Context(), id, t)
		audit.record(c, id, "llm_models.set", auditTarget(t), outcome(err))
		switch {
		case errors.Is(err, store.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		case err != nil:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save LLM models"})
		default:
			respond(c, id, t)
		}
	})
}

// healthzPayload is the body of GET /healthz. llm keeps its format and
// llm_models adds the model of each task and the fallback chain (R16).
func healthzPayload(version, color string, ai aiStack, lexical, database, authMode string) gin.H {
	return gin.H{
		"status":       "healthy",
		"version":      version,
		"color":        color,
		"llm":          ai.llmMode,
		"llm_models":   modelsView(ai.models.defaults),
		"vector_store": ai.vectorStoreName(),
		"embedder":     ai.embedder,
		"lexical_rag":  lexical,
		"database":     database,
		"auth_mode":    authMode,
	}
}
