package telemetry

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/martinllanos/only-ai-pods/internal/cmmi"
)

// MetricsCollector holds the metrics exported in Prometheus format
// (SPEC-CORE-25, SPEC-CORE-68). Every number comes from something measured:
// pod requests, rate-limit blocks, the semantic cache and the CMMI report
// generated from git when the image was built.
type MetricsCollector struct {
	mu                  sync.RWMutex
	podRequests         int64
	podDurationUs       int64
	rateLimitsExceeded  int64
	ragFallbacks        int64
	requestsByPodStatus map[string]int64
	afipByIntent        map[string]int64

	cacheStats func() (hits, misses, purged int64)
	podCount   func() int
	cmmi       *cmmi.Report
}

var globalCollector *MetricsCollector
var once sync.Once

func GetMetricsCollector() *MetricsCollector {
	once.Do(func() { globalCollector = NewMetricsCollector() })
	return globalCollector
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{requestsByPodStatus: make(map[string]int64), afipByIntent: make(map[string]int64)}
}

// SetSources wires the live sources that the collector reads on demand.
func (m *MetricsCollector) SetSources(cacheStats func() (int64, int64, int64), podCount func() int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cacheStats, m.podCount = cacheStats, podCount
}

// SetCMMI sets the CMMI report; nil means no report was found.
func (m *MetricsCollector) SetCMMI(r *cmmi.Report) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cmmi = r
}

// RecordRequest records one AI Pod request (chat) with its latency and HTTP status.
func (m *MetricsCollector) RecordRequest(podID string, duration time.Duration, statusCode int) {
	if podID == "" {
		podID = "none"
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.podRequests++
	m.podDurationUs += duration.Microseconds()
	m.requestsByPodStatus[fmt.Sprintf("%s:%d", podID, statusCode)]++
}

func (m *MetricsCollector) RecordRateLimitExceeded() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rateLimitsExceeded++
}

// RecordRAGFallback counts searches that degraded from external embeddings to
// query expansion + lexical search (SPEC-CORE-70.R4).
func (m *MetricsCollector) RecordRAGFallback() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ragFallbacks++
}

// AFIP query intents exported by aipods_afip_queries_total (SPEC-CORE-72.R4).
// Only these labels are accepted, so the metric cardinality stays fixed.
var afipIntents = []string{"informational", "operational"}

// RecordAFIPIntent counts one AFIP pod query by intent (SPEC-CORE-72.R4).
// Unknown intents are ignored.
func (m *MetricsCollector) RecordAFIPIntent(intent string) {
	known := false
	for _, i := range afipIntents {
		known = known || i == intent
	}
	if !known {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.afipByIntent[intent]++
}

// RecordPodIntent is the router hook (SetIntentHook): it forwards the
// intents reported by the AFIP pod.
func (m *MetricsCollector) RecordPodIntent(podID, intent string) {
	if podID == afipPodID {
		m.RecordAFIPIntent(intent)
	}
}

const afipPodID = "POD_AFIP_FISCAL"

// Snapshot is what the admin telemetry endpoint returns.
type Snapshot struct {
	PodRequests      int64            `json:"pod_requests"`
	AvgLatencyMs     float64          `json:"avg_latency_ms"`
	CacheHits        int64            `json:"cache_hits"`
	CacheMisses      int64            `json:"cache_misses"`
	CachePurged      int64            `json:"cache_purged"`
	RateLimitBlocked int64            `json:"rate_limit_blocked"`
	RAGFallbacks     int64            `json:"rag_fallbacks"`
	ActivePods       int              `json:"active_pods_count"`
	ByPodStatus      map[string]int64 `json:"requests_by_pod_status"`
	// AFIPByIntent counts AFIP pod queries by intent (SPEC-CORE-72.R4).
	AFIPByIntent map[string]int64 `json:"afip_queries_by_intent"`
	CMMI         *cmmi.Report     `json:"cmmi"`
}

func (m *MetricsCollector) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := Snapshot{
		PodRequests:      m.podRequests,
		RateLimitBlocked: m.rateLimitsExceeded,
		RAGFallbacks:     m.ragFallbacks,
		ByPodStatus:      make(map[string]int64, len(m.requestsByPodStatus)),
		AFIPByIntent:     make(map[string]int64, len(afipIntents)),
		CMMI:             m.cmmi,
	}
	if m.podRequests > 0 {
		s.AvgLatencyMs = float64(int64(float64(m.podDurationUs)/1000/float64(m.podRequests)*100+0.5)) / 100
	}
	for k, v := range m.requestsByPodStatus {
		s.ByPodStatus[k] = v
	}
	for _, i := range afipIntents {
		s.AFIPByIntent[i] = m.afipByIntent[i]
	}
	if m.cacheStats != nil {
		s.CacheHits, s.CacheMisses, s.CachePurged = m.cacheStats()
	}
	if m.podCount != nil {
		s.ActivePods = m.podCount()
	}
	return s
}

// PrometheusExporterHandler serves GET /metrics.
func (m *MetricsCollector) PrometheusExporterHandler(c *gin.Context) {
	c.Data(http.StatusOK, "text/plain; version=0.0.4", []byte(m.Prometheus()))
}

func (m *MetricsCollector) Prometheus() string {
	s := m.Snapshot()
	var b strings.Builder
	metric := func(name, typ, help string, v any) {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s %s\n%s %v\n\n", name, help, name, typ, name, v)
	}
	metric("aipods_requests_total", "counter", "AI Pod requests processed (chat).", s.PodRequests)
	metric("aipods_rag_fallback_total", "counter", "RAG searches that fell back from external embeddings to lexical search.", s.RAGFallbacks)
	metric("aipods_request_duration_avg_ms", "gauge", "Average AI Pod request latency in milliseconds.", fmt.Sprintf("%.2f", s.AvgLatencyMs))
	keys := make([]string, 0, len(s.ByPodStatus))
	for k := range s.ByPodStatus {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) > 0 {
		b.WriteString("# HELP aipods_requests_by_pod_total AI Pod requests by pod and HTTP status.\n# TYPE aipods_requests_by_pod_total counter\n")
		for _, k := range keys {
			pod, status, _ := strings.Cut(k, ":")
			fmt.Fprintf(&b, "aipods_requests_by_pod_total{pod=%q,status=%q} %d\n", pod, status, s.ByPodStatus[k])
		}
		b.WriteString("\n")
	}
	b.WriteString("# HELP aipods_afip_queries_total AFIP pod queries by intent (informational or operational).\n# TYPE aipods_afip_queries_total counter\n")
	for _, i := range afipIntents {
		fmt.Fprintf(&b, "aipods_afip_queries_total{intent=%q} %d\n", i, s.AFIPByIntent[i])
	}
	b.WriteString("\n")
	metric("aipods_cache_hits_total", "counter", "Semantic cache hits.", s.CacheHits)
	metric("aipods_cache_misses_total", "counter", "Semantic cache misses.", s.CacheMisses)
	hitRatio := 0.0
	if t := s.CacheHits + s.CacheMisses; t > 0 {
		hitRatio = float64(s.CacheHits) / float64(t) * 100
	}
	metric("aipods_cache_hit_ratio_percent", "gauge", "Percentage of cache hits.", fmt.Sprintf("%.2f", hitRatio))
	metric("aipods_rate_limit_exceeded_total", "counter", "Requests blocked by the rate limiter (HTTP 429).", s.RateLimitBlocked)
	metric("aipods_active_pods", "gauge", "AI Pods registered in the router.", s.ActivePods)

	// CMMI metrics only when the build produced a report (SPEC-CORE-68).
	if r := s.CMMI; r != nil {
		fmt.Fprintf(&b, "# HELP aipods_cmmi_report_info Source of the CMMI metrics: git history at build time.\n# TYPE aipods_cmmi_report_info gauge\naipods_cmmi_report_info{commit=%q,generated_at=%q,shallow=%q} 1\n\n", r.Commit, r.GeneratedAt, fmt.Sprint(r.Shallow))
		metric("aipods_cmmi_commits_total", "gauge", "Commits in the analysed history.", r.CommitsTotal)
		metric("aipods_cmmi_level4_spec_traceability_index", "gauge", "Share of commits that reference a SPEC, RFC or issue (0 to 1).", fmt.Sprintf("%.4f", r.TraceabilityIndex))
		metric("aipods_cmmi_level4_defect_density_per_kloc", "gauge", "Fix commits per 1000 lines of non-test Go code.", fmt.Sprintf("%.4f", r.DefectDensity))
		metric("aipods_cmmi_level4_avg_spec_lead_time_hours", "gauge", "Average hours from first to last commit of specs with two or more commits.", fmt.Sprintf("%.2f", r.AvgSpecLeadTimeHrs))
	}
	return b.String()
}
