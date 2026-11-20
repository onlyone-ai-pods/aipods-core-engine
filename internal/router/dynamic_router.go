package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/martinllanos/only-ai-pods/internal/llm"
	"github.com/martinllanos/only-ai-pods/internal/rag"

	"github.com/martinllanos/only-ai-pods/internal/pod"
	"github.com/martinllanos/only-ai-pods/internal/pod/afip"
	"github.com/martinllanos/only-ai-pods/internal/pod/billing"
	"github.com/martinllanos/only-ai-pods/internal/pod/evocrm"
	"github.com/martinllanos/only-ai-pods/internal/pod/gdrive"
	githubdevops "github.com/martinllanos/only-ai-pods/internal/pod/github_devops"
	"github.com/martinllanos/only-ai-pods/internal/pod/multimedia"
	"github.com/martinllanos/only-ai-pods/internal/pod/sap"
	"github.com/martinllanos/only-ai-pods/internal/pod/scm"
	securitypod "github.com/martinllanos/only-ai-pods/internal/pod/security"
)

// DynamicPodConfig represents a Pod registered dynamically via Database / API
type DynamicPodConfig struct {
	PodID       string   `json:"pod_id"`
	Name        string   `json:"name"`
	TenantID    string   `json:"tenant_id"`
	EndpointURL string   `json:"endpoint_url"`
	Keywords    []string `json:"keywords"`
	Status      string   `json:"status"`
}

// HTTPSidecarAdapter adapts any external HTTP Microservice / Sidecar Pod into a pod.BaseAIPod
type HTTPSidecarAdapter struct {
	config DynamicPodConfig
	client *http.Client
}

func NewHTTPSidecarAdapter(config DynamicPodConfig) *HTTPSidecarAdapter {
	return &HTTPSidecarAdapter{
		config: config,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (h *HTTPSidecarAdapter) ID() string   { return h.config.PodID }
func (h *HTTPSidecarAdapter) Name() string { return h.config.Name }

func (h *HTTPSidecarAdapter) ProcessQuery(ctx context.Context, tenantID string, query string, dryRun bool) (*pod.PodResponse, error) {
	reqBody, err := json.Marshal(map[string]interface{}{
		"tenant_id": tenantID,
		"query":     query,
		"dry_run":   dryRun,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", h.config.EndpointURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return &pod.PodResponse{
			PodID:  h.config.PodID,
			Answer: fmt.Sprintf("AI Pod Dinámico (%s): Respuesta procesada en microservicio externo (%s).", h.config.Name, h.config.EndpointURL),
			Status: "SUCCESS",
		}, nil
	}
	defer resp.Body.Close()

	var podResp pod.PodResponse
	if err := json.NewDecoder(resp.Body).Decode(&podResp); err != nil {
		return nil, err
	}

	return &podResp, nil
}

// Retriever returns the tenant's relevant document chunks (implemented by rag.RAGIngestionService).
type Retriever interface {
	Search(ctx context.Context, query string, topK int) ([]rag.SearchResult, error)
}

// DynamicSmartRouter manages both static compiled Core Pods and dynamically registered HTTP/DB Pods
type DynamicSmartRouter struct {
	mu           sync.RWMutex
	staticPods   map[string]pod.BaseAIPod
	dynamicPods  map[string]DynamicPodConfig
	httpAdapters map[string]*HTTPSidecarAdapter

	// Optional real LLM + RAG (SPEC-CORE-66). Nil means prototype mode.
	llm       llm.Client
	retriever Retriever

	// Dual RAG strategy per tenant (SPEC-CORE-70).
	lexical       LexicalRetriever
	strategy      StrategyResolver
	onFallback    func()
	vectorBreaker *CircuitBreaker

	// Model per task and per tenant (SPEC-CORE-74). Nil means every task uses
	// the base client as before.
	llmRuntime    *llm.Runtime
	modelResolver ModelResolver

	// Semantic cache (SPEC-CORE-71). Nil disables it.
	cache ResponseCache

	// topK is how many chunks a RAG search returns (SPEC-CORE-72.R6).
	topK int
	// onIntent receives the intent a pod reported (SPEC-CORE-72.R4).
	onIntent func(podID, intent string)
}

// ResponseCache is the semantic cache seen by the router (SPEC-CORE-71).
type ResponseCache interface {
	GetCachedResponse(ctx context.Context, tenantID, query string) (*pod.PodResponse, bool)
	StoreResponse(ctx context.Context, tenantID, query string, resp *pod.PodResponse, global bool)
}

// globalCachePods are the pods whose answers are about public regulations
// (ARCA manuals and tax rules) and may be shared across tenants (SPEC-CORE-71.R2).
var globalCachePods = map[string]bool{"POD_AFIP_FISCAL": true}

// SetCache enables the semantic cache. Pass nil to disable it.
func (r *DynamicSmartRouter) SetCache(c ResponseCache) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache = c
}

func NewDynamicSmartRouter() *DynamicSmartRouter {
	r := &DynamicSmartRouter{
		staticPods:    make(map[string]pod.BaseAIPod),
		dynamicPods:   make(map[string]DynamicPodConfig),
		httpAdapters:  make(map[string]*HTTPSidecarAdapter),
		vectorBreaker: NewCircuitBreaker(breakerFailures, breakerReset),
		topK:          DefaultRAGTopK,
	}

	// Register Core Essential Static Pods
	r.RegisterStaticPod(afip.NewAFIPPod())
	r.RegisterStaticPod(billing.NewCoreBillingPod())
	r.RegisterStaticPod(securitypod.NewCoreSecurityAuditPod())

	// Register Domain & Multimodal Pods
	r.RegisterStaticPod(githubdevops.NewGitHubDevOpsPod())
	r.RegisterStaticPod(sap.NewSAPPod())
	r.RegisterStaticPod(evocrm.NewEvoCRMPod())
	r.RegisterStaticPod(scm.NewSCMPod())
	r.RegisterStaticPod(multimedia.NewMultimediaWhisperPod())
	r.RegisterStaticPod(gdrive.NewGDriveSyncPod())

	return r
}

// SetIntentHook is called with the intent each pod execution reports, e.g.
// the AFIP pod's informational/operational reading (SPEC-CORE-72.R4). Cache
// hits do not run the pod and are not reported.
func (r *DynamicSmartRouter) SetIntentHook(f func(podID, intent string)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onIntent = f
}

// SetLLM enables LLM routing and answer drafting. Pass nil to disable.
// PodCount reports how many pods (static and dynamic) are registered.
func (r *DynamicSmartRouter) PodCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.staticPods) + len(r.dynamicPods)
}

func (r *DynamicSmartRouter) SetLLM(c llm.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.llm = c
}

// ModelResolver returns the effective LLM models of a tenant: its own
// configuration over the platform's (SPEC-CORE-74.R6).
type ModelResolver func(ctx context.Context, tenantID string) llm.Models

// SetLLMRuntime enables a model per task with its fallback chain
// (SPEC-CORE-74). resolver may be nil: every tenant then uses rt.Defaults.
func (r *DynamicSmartRouter) SetLLMRuntime(rt *llm.Runtime, resolver ModelResolver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.llmRuntime = rt
	r.modelResolver = resolver
}

// taskClients are the LLM clients of one query, one per task.
type taskClients struct {
	classify llm.Client
	expand   llm.Client
	answer   llm.Client
}

// clientsFor binds the base client to each task with the tenant's models.
// The tenant's configuration is read once per query, after the cache miss
// (SPEC-CORE-74.R7). platformAnswer is the answer model of the platform, or
// "" when no runtime is configured.
func clientsFor(ctx context.Context, base llm.Client, rt *llm.Runtime, resolver ModelResolver, tenantID string) (tc taskClients, platformAnswer string) {
	if base == nil || rt == nil {
		return taskClients{classify: base, expand: base, answer: base}, ""
	}
	models := rt.Defaults
	if resolver != nil {
		models = resolver(ctx, tenantID)
	}
	platformAnswer = rt.Defaults.Answer
	if platformAnswer == "" {
		platformAnswer = base.Model()
	}
	return taskClients{
		classify: rt.Client(base, llm.TaskClassify, models),
		expand:   rt.Client(base, llm.TaskExpand, models),
		answer:   rt.Client(base, llm.TaskAnswer, models),
	}, platformAnswer
}

// SetRetriever enables RAG context for drafted answers.
func (r *DynamicSmartRouter) SetRetriever(ret Retriever) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.retriever = ret
}

func (r *DynamicSmartRouter) RegisterStaticPod(p pod.BaseAIPod) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.staticPods[p.ID()] = p
}

// RegisterDynamicPod allows clients to register new AI Pods dynamically AT RUNTIME without recompiling Go core
func (r *DynamicSmartRouter) RegisterDynamicPod(config DynamicPodConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dynamicPods[config.PodID] = config
	r.httpAdapters[config.PodID] = NewHTTPSidecarAdapter(config)
}

// SyncDynamicPods registers the pods that are new or changed in configs, as
// read from the store by every instance (SPEC-CORE-73.R10). Pods are never
// removed here: there is no unregister API. It returns how many it registered.
func (r *DynamicSmartRouter) SyncDynamicPods(configs []DynamicPodConfig) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, cfg := range configs {
		if old, ok := r.dynamicPods[cfg.PodID]; ok && sameDynamicPod(old, cfg) {
			continue
		}
		r.dynamicPods[cfg.PodID] = cfg
		r.httpAdapters[cfg.PodID] = NewHTTPSidecarAdapter(cfg)
		n++
	}
	return n
}

func sameDynamicPod(a, b DynamicPodConfig) bool {
	if a.Name != b.Name || a.TenantID != b.TenantID || a.EndpointURL != b.EndpointURL ||
		a.Status != b.Status || len(a.Keywords) != len(b.Keywords) {
		return false
	}
	for i := range a.Keywords {
		if a.Keywords[i] != b.Keywords[i] {
			return false
		}
	}
	return true
}

// RouteAndExecute picks a pod, runs it and, when an LLM is configured, drafts
// the final answer from the pod result and the tenant's documents.
//
// In simulation mode (dryRun) it first asks the semantic cache, tenant
// partition and then GLOBAL; a hit skips the LLM and the search (SPEC-CORE-71.R1).
func (r *DynamicSmartRouter) RouteAndExecute(ctx context.Context, tenantID, query string, dryRun bool) (*pod.PodResponse, error) {
	r.mu.RLock()
	cache := r.cache
	r.mu.RUnlock()
	if cache != nil && dryRun {
		if hit, ok := cache.GetCachedResponse(ctx, tenantID, query); ok {
			return hit, nil
		}
	}

	r.mu.RLock()
	client := r.llm
	rt, resolver := r.llmRuntime, r.modelResolver
	r.mu.RUnlock()
	clients, platformAnswer := clientsFor(ctx, client, rt, resolver, tenantID)

	r.mu.RLock()
	deps := retrieval{vector: r.retriever, lexical: r.lexical, strategy: r.strategy, topK: r.topK}
	target := r.selectPod(ctx, clients.classify, tenantID, query)
	r.mu.RUnlock()

	res, err := target.ProcessQuery(ctx, tenantID, query, dryRun)
	if err == nil && res != nil && res.Intent != "" {
		r.mu.RLock()
		hook := r.onIntent
		r.mu.RUnlock()
		if hook != nil {
			hook(res.PodID, res.Intent)
		}
	}
	if err != nil || client == nil || res == nil {
		return res, err
	}
	out, globalSources := r.draftAnswer(ctx, clients, deps, tenantID, query, res)
	if cache != nil && cacheable(out, dryRun) {
		global := globalSources && globalCachePods[out.PodID] &&
			!strings.Contains(strings.ToLower(out.Answer), strings.ToLower(tenantID))
		// GLOBAL only takes answers drafted by the platform's answer model: a
		// tenant's own model or a fallback stays in the tenant's partition
		// (SPEC-CORE-74.R14).
		if platformAnswer != "" && out.Model != platformAnswer {
			global = false
		}
		cache.StoreResponse(ctx, tenantID, query, out, global)
	}
	return out, nil
}

// cacheable keeps out of the cache anything that is not a plain drafted
// answer: actions (dry-run or approvals), non-simulated runs and answers the
// LLM did not write (SPEC-CORE-71.R3).
func cacheable(res *pod.PodResponse, dryRun bool) bool {
	return dryRun && res != nil && res.DryRunResult == nil && res.Model != "" && res.Answer != ""
}

// selectPod must be called with r.mu held for reading.
func (r *DynamicSmartRouter) selectPod(ctx context.Context, client llm.Client, tenantID, query string) pod.BaseAIPod {
	if client != nil {
		options, lookup := r.candidates(tenantID)
		id, err := llm.Classify(ctx, client, query, options)
		if err == nil {
			return lookup[id]
		}
		log.Printf("[router] LLM classification failed, using keywords: %v", err)
	}
	return r.selectByKeyword(tenantID, query)
}

// candidates lists the pods this tenant can reach, sorted for a stable prompt.
func (r *DynamicSmartRouter) candidates(tenantID string) ([]llm.Option, map[string]pod.BaseAIPod) {
	lookup := make(map[string]pod.BaseAIPod)
	var options []llm.Option
	for id, p := range r.staticPods {
		lookup[id] = p
		options = append(options, llm.Option{ID: id, Description: p.Name()})
	}
	for id, cfg := range r.dynamicPods {
		if cfg.TenantID != "GLOBAL" && cfg.TenantID != tenantID {
			continue
		}
		lookup[id] = r.httpAdapters[id]
		desc := cfg.Name
		if len(cfg.Keywords) > 0 {
			desc += " (temas: " + strings.Join(cfg.Keywords, ", ") + ")"
		}
		options = append(options, llm.Option{ID: id, Description: desc})
	}
	sort.Slice(options, func(i, j int) bool { return options[i].ID < options[j].ID })
	return options, lookup
}

// selectByKeyword is the prototype router, kept as a fallback (SPEC-CORE-66.R2).
func (r *DynamicSmartRouter) selectByKeyword(tenantID, query string) pod.BaseAIPod {
	lowerQuery := strings.ToLower(query)

	// 1. Check Dynamic Registered Pods (Database / Runtime Plugins)
	for podID, config := range r.dynamicPods {
		if config.TenantID != "GLOBAL" && config.TenantID != tenantID {
			continue
		}
		for _, kw := range config.Keywords {
			if strings.Contains(lowerQuery, strings.ToLower(kw)) {
				return r.httpAdapters[podID]
			}
		}
	}

	// 2. Check Static Compiled Core Pods
	containsAny := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(lowerQuery, w) {
				return true
			}
		}
		return false
	}
	switch {
	case containsAny("audio", "video", "transcribir", "mp3", "mp4"):
		return r.staticPods["POD_MULTIMEDIA_WHISPER"]
	case containsAny("gdrive", "google drive", "gdoc"):
		return r.staticPods["POD_GDRIVE_SYNC"]
	case containsAny("factura", "saldo", "estado de cuenta"):
		return r.staticPods["POD_CORE_BILLING_ODOO"]
	case containsAny("auditoria", "soc2", "seguridad"):
		return r.staticPods["POD_CORE_SECURITY_AUDIT"]
	case containsAny("scm", "wms", "mrp", "bom", "landed", "flete"):
		return r.staticPods["POD_SCM_LOGISTICS"]
	case containsAny("evocrm", "helpdesk", "whatsapp", "ticket"):
		return r.staticPods["POD_EVOCRM_HELPDESK"]
	case containsAny("sap", "s4hana", "odata", "b1"):
		return r.staticPods["POD_SAP_ENTERPRISE"]
	case containsAny("github", "odoo.sh", "repo", "despliegue"):
		return r.staticPods["POD_GITHUB_DEVOPS"]
	}

	// Default Fallback Pod
	return r.staticPods["POD_AFIP_FISCAL"]
}
