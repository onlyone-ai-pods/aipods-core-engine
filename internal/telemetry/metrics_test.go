package telemetry

import (
	"strings"
	"testing"
	"time"

	"github.com/martinllanos/only-ai-pods/internal/cmmi"
)

func TestPrometheusUsesMeasuredValues(t *testing.T) {
	m := NewMetricsCollector()
	m.SetSources(func() (int64, int64, int64) { return 3, 1, 0 }, func() int { return 9 })
	m.RecordRequest("POD_AFIP_FISCAL", 100*time.Millisecond, 200)
	m.RecordRequest("POD_AFIP_FISCAL", 300*time.Millisecond, 200)
	m.RecordRateLimitExceeded()

	out := m.Prometheus()
	for _, want := range []string{
		"aipods_requests_total 2",
		"aipods_request_duration_avg_ms 200.00",
		`aipods_requests_by_pod_total{pod="POD_AFIP_FISCAL",status="200"} 2`,
		"aipods_cache_hit_ratio_percent 75.00",
		"aipods_rate_limit_exceeded_total 1",
		"aipods_active_pods 9",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "cmmi") {
		t.Error("without a report, no CMMI metric may be exported")
	}

	m.SetCMMI(&cmmi.Report{Commit: "abc1234", CommitsTotal: 8, TraceabilityIndex: 1, DefectDensity: 0.39, AvgSpecLeadTimeHrs: 6.46})
	out = m.Prometheus()
	for _, want := range []string{
		`aipods_cmmi_report_info{commit="abc1234"`,
		"aipods_cmmi_level4_spec_traceability_index 1.0000",
		"aipods_cmmi_level4_defect_density_per_kloc 0.3900",
		"aipods_cmmi_level4_avg_spec_lead_time_hours 6.46",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
}

// SPEC-CORE-72.R4: AFIP queries are counted by intent in /metrics and in the
// admin telemetry snapshot; other pods and unknown intents are ignored.
func TestSPEC_CORE_72_R4_AFIPIntentCounter(t *testing.T) {
	m := NewMetricsCollector()
	out := m.Prometheus()
	for _, want := range []string{
		`aipods_afip_queries_total{intent="informational"} 0`,
		`aipods_afip_queries_total{intent="operational"} 0`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q before any query", want)
		}
	}

	m.RecordPodIntent("POD_AFIP_FISCAL", "informational")
	m.RecordPodIntent("POD_AFIP_FISCAL", "operational")
	m.RecordPodIntent("POD_SAP_ENTERPRISE", "operational")
	m.RecordAFIPIntent("bogus")

	out = m.Prometheus()
	for _, want := range []string{
		"# TYPE aipods_afip_queries_total counter",
		`aipods_afip_queries_total{intent="informational"} 1`,
		`aipods_afip_queries_total{intent="operational"} 1`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "bogus") {
		t.Error("unknown intents must not create series")
	}
	s := m.Snapshot()
	if s.AFIPByIntent["informational"] != 1 || s.AFIPByIntent["operational"] != 1 || len(s.AFIPByIntent) != 2 {
		t.Errorf("snapshot = %v", s.AFIPByIntent)
	}
}
