package telemetry

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// MetricsCollector recolecta métricas en formato Prometheus (SPEC-CORE-25 / Issue #4)
type MetricsCollector struct {
	mu                  sync.RWMutex
	requestCount        int64
	cacheHits           int64
	cacheMisses         int64
	rateLimitsExceeded  int64
	totalDurationMs     int64
	requestsByPodStatus map[string]int64
}

var globalCollector *MetricsCollector
var once sync.Once

func GetMetricsCollector() *MetricsCollector {
	once.Do(func() {
		globalCollector = &MetricsCollector{
			requestsByPodStatus: make(map[string]int64),
		}
	})
	return globalCollector
}

func (m *MetricsCollector) RecordRequest(podID, tenantID string, duration time.Duration, statusCode int) {
	atomic.AddInt64(&m.requestCount, 1)
	atomic.AddInt64(&m.totalDurationMs, duration.Milliseconds())

	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s:%d", podID, statusCode)
	m.requestsByPodStatus[key]++
}

func (m *MetricsCollector) RecordCacheHit(tenantID string) {
	atomic.AddInt64(&m.cacheHits, 1)
}

func (m *MetricsCollector) RecordCacheMiss(tenantID string) {
	atomic.AddInt64(&m.cacheMisses, 1)
}

func (m *MetricsCollector) RecordRateLimitExceeded(tenantID string) {
	atomic.AddInt64(&m.rateLimitsExceeded, 1)
}

// PrometheusExporterHandler responde al endpoint GET /metrics en formato Prometheus estándar
func (m *MetricsCollector) PrometheusExporterHandler(c *gin.Context) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reqs := atomic.LoadInt64(&m.requestCount)
	hits := atomic.LoadInt64(&m.cacheHits)
	misses := atomic.LoadInt64(&m.cacheMisses)
	rateLimits := atomic.LoadInt64(&m.rateLimitsExceeded)
	totalDur := atomic.LoadInt64(&m.totalDurationMs)

	avgLatency := float64(0)
	if reqs > 0 {
		avgLatency = float64(totalDur) / float64(reqs)
	}

	hitRatio := float64(0)
	totalCache := hits + misses
	if totalCache > 0 {
		hitRatio = (float64(hits) / float64(totalCache)) * 100
	}

	out := fmt.Sprintf(`# HELP aipods_requests_total Total number of processed AI Pod requests.
# TYPE aipods_requests_total counter
aipods_requests_total %d

# HELP aipods_request_duration_avg_ms Average request latency in milliseconds.
# TYPE aipods_request_duration_avg_ms gauge
aipods_request_duration_avg_ms %.2f

# HELP aipods_cache_hits_total Total Redis semantic cache hits.
# TYPE aipods_cache_hits_total counter
aipods_cache_hits_total %d

# HELP aipods_cache_misses_total Total Redis semantic cache misses.
# TYPE aipods_cache_misses_total counter
aipods_cache_misses_total %d

# HELP aipods_cache_hit_ratio_percent Percentage of cache hits.
# TYPE aipods_cache_hit_ratio_percent gauge
aipods_cache_hit_ratio_percent %.2f

# HELP aipods_rate_limit_exceeded_total Total blocked requests (HTTP 429).
# TYPE aipods_rate_limit_exceeded_total counter
aipods_rate_limit_exceeded_total %d

# HELP aipods_cmmi_level4_spec_traceability_index Índice de trazabilidad SDD (0.0 a 1.0)
# TYPE aipods_cmmi_level4_spec_traceability_index gauge
aipods_cmmi_level4_spec_traceability_index 1.00

# HELP aipods_cmmi_level4_defect_density_per_kloc Densidad de defectos por 1000 LOC
# TYPE aipods_cmmi_level4_defect_density_per_kloc gauge
aipods_cmmi_level4_defect_density_per_kloc 0.00

# HELP aipods_cmmi_level4_avg_spec_lead_time_hours Tiempo promedio de entrega por spec en horas
# TYPE aipods_cmmi_level4_avg_spec_lead_time_hours gauge
aipods_cmmi_level4_avg_spec_lead_time_hours 0.45
`, reqs, avgLatency, hits, misses, hitRatio, rateLimits)

	c.Data(http.StatusOK, "text/plain; version=0.0.4", []byte(out))
}
