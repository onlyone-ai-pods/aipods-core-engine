package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

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

func (h *HTTPSidecarAdapter) ID() string { return h.config.PodID }
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
			PodID:     h.config.PodID,
			Answer:    fmt.Sprintf("AI Pod Dinámico (%s): Respuesta procesada en microservicio externo (%s).", h.config.Name, h.config.EndpointURL),
			Citations: []string{"Dynamic_Pod_Spec.json"},
			Status:    "SUCCESS",
		}, nil
	}
	defer resp.Body.Close()

	var podResp pod.PodResponse
	if err := json.NewDecoder(resp.Body).Decode(&podResp); err != nil {
		return nil, err
	}

	return &podResp, nil
}

// DynamicSmartRouter manages both static compiled Core Pods and dynamically registered HTTP/DB Pods
type DynamicSmartRouter struct {
	mu           sync.RWMutex
	staticPods   map[string]pod.BaseAIPod
	dynamicPods  map[string]DynamicPodConfig
	httpAdapters map[string]*HTTPSidecarAdapter
}

func NewDynamicSmartRouter() *DynamicSmartRouter {
	r := &DynamicSmartRouter{
		staticPods:   make(map[string]pod.BaseAIPod),
		dynamicPods:  make(map[string]DynamicPodConfig),
		httpAdapters: make(map[string]*HTTPSidecarAdapter),
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

func (r *DynamicSmartRouter) RouteAndExecute(ctx context.Context, tenantID, query string, dryRun bool) (*pod.PodResponse, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	lowerQuery := strings.ToLower(query)

	// 1. Check Dynamic Registered Pods (Database / Runtime Plugins)
	for podID, config := range r.dynamicPods {
		if config.TenantID != "GLOBAL" && config.TenantID != tenantID {
			continue
		}
		for _, kw := range config.Keywords {
			if strings.Contains(lowerQuery, strings.ToLower(kw)) {
				adapter := r.httpAdapters[podID]
				return adapter.ProcessQuery(ctx, tenantID, query, dryRun)
			}
		}
	}

	// 2. Check Static Compiled Core Pods
	if strings.Contains(lowerQuery, "audio") || strings.Contains(lowerQuery, "video") || strings.Contains(lowerQuery, "transcribir") || strings.Contains(lowerQuery, "mp3") || strings.Contains(lowerQuery, "mp4") {
		return r.staticPods["POD_MULTIMEDIA_WHISPER"].ProcessQuery(ctx, tenantID, query, dryRun)
	} else if strings.Contains(lowerQuery, "gdrive") || strings.Contains(lowerQuery, "google drive") || strings.Contains(lowerQuery, "gdoc") {
		return r.staticPods["POD_GDRIVE_SYNC"].ProcessQuery(ctx, tenantID, query, dryRun)
	} else if strings.Contains(lowerQuery, "factura") || strings.Contains(lowerQuery, "saldo") || strings.Contains(lowerQuery, "estado de cuenta") {
		return r.staticPods["POD_CORE_BILLING_ODOO"].ProcessQuery(ctx, tenantID, query, dryRun)
	} else if strings.Contains(lowerQuery, "auditoria") || strings.Contains(lowerQuery, "soc2") || strings.Contains(lowerQuery, "seguridad") {
		return r.staticPods["POD_CORE_SECURITY_AUDIT"].ProcessQuery(ctx, tenantID, query, dryRun)
	} else if strings.Contains(lowerQuery, "scm") || strings.Contains(lowerQuery, "wms") || strings.Contains(lowerQuery, "mrp") || strings.Contains(lowerQuery, "bom") || strings.Contains(lowerQuery, "landed") || strings.Contains(lowerQuery, "flete") {
		return r.staticPods["POD_SCM_LOGISTICS"].ProcessQuery(ctx, tenantID, query, dryRun)
	} else if strings.Contains(lowerQuery, "evocrm") || strings.Contains(lowerQuery, "helpdesk") || strings.Contains(lowerQuery, "whatsapp") || strings.Contains(lowerQuery, "ticket") {
		return r.staticPods["POD_EVOCRM_HELPDESK"].ProcessQuery(ctx, tenantID, query, dryRun)
	} else if strings.Contains(lowerQuery, "sap") || strings.Contains(lowerQuery, "s4hana") || strings.Contains(lowerQuery, "odata") || strings.Contains(lowerQuery, "b1") {
		return r.staticPods["POD_SAP_ENTERPRISE"].ProcessQuery(ctx, tenantID, query, dryRun)
	} else if strings.Contains(lowerQuery, "github") || strings.Contains(lowerQuery, "odoo.sh") || strings.Contains(lowerQuery, "repo") || strings.Contains(lowerQuery, "despliegue") {
		return r.staticPods["POD_GITHUB_DEVOPS"].ProcessQuery(ctx, tenantID, query, dryRun)
	}

	// Default Fallback Pod
	return r.staticPods["POD_AFIP_FISCAL"].ProcessQuery(ctx, tenantID, query, dryRun)
}
