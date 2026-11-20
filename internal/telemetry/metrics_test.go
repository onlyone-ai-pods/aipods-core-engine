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

func TestSPEC_CORE_73_R6_BuildInfo(t *testing.T) {
	m := NewMetricsCollector()
	if strings.Contains(m.Prometheus(), "aipods_build_info") {
		t.Fatal("build info published before SetBuildInfo")
	}
	m.SetBuildInfo("41.0.0-abc1234", "green")
	if out := m.Prometheus(); !strings.Contains(out, `aipods_build_info{version="41.0.0-abc1234",color="green"} 1`) {
		t.Fatalf("missing build info line:\n%s", out)
	}
	if s := m.Snapshot(); s.BuildVersion != "41.0.0-abc1234" || s.BuildColor != "green" {
		t.Fatalf("snapshot = %+v", s)
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

// SPEC-CORE-72.R9: OR fills of the lexical index are counted in /metrics and
// in the admin telemetry snapshot.
func TestSPEC_CORE_72_R9_LexicalORFallbackCounter(t *testing.T) {
	m := NewMetricsCollector()
	if out := m.Prometheus(); !strings.Contains(out, "aipods_rag_lexical_or_fallback_total 0") {
		t.Fatalf("missing counter before any fill:\n%s", out)
	}
	m.RecordLexicalORFallback()
	m.RecordLexicalORFallback()
	if out := m.Prometheus(); !strings.Contains(out, "# TYPE aipods_rag_lexical_or_fallback_total counter\naipods_rag_lexical_or_fallback_total 2") {
		t.Errorf("counter not exported:\n%s", out)
	}
	if s := m.Snapshot(); s.LexicalORFallbacks != 2 {
		t.Errorf("snapshot = %d, want 2", s.LexicalORFallbacks)
	}
}

// SPEC-CORE-74.R15: LLM calls and fallback hops with bounded labels.
func TestSPEC_CORE_74_R15_LLMMetrics(t *testing.T) {
	m := NewMetricsCollector()
	m.SetLLMModels([]string{"claude-opus-5-5", "claude-sonnet-5-5"})
	m.RecordLLMRequest("answer", "claude-opus-5-5", "error")
	m.RecordLLMRequest("answer", "claude-sonnet-5-5", "ok")
	m.RecordLLMRequest("answer", "claude-sonnet-5-5", "ok")
	m.RecordLLMFallback("answer", "claude-opus-5-5", "claude-sonnet-5-5")
	// Values outside the known sets never become labels.
	m.RecordLLMRequest("tenant ACME", "consulta del cliente", "boom")

	out := m.Prometheus()
	for _, want := range []string{
		`aipods_llm_requests_total{task="answer",model="claude-opus-5-5",outcome="error"} 1`,
		`aipods_llm_requests_total{task="answer",model="claude-sonnet-5-5",outcome="ok"} 2`,
		`aipods_llm_fallback_total{task="answer",from="claude-opus-5-5",to="claude-sonnet-5-5"} 1`,
		`aipods_llm_requests_total{task="other",model="other",outcome="other"} 1`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "ACME") || strings.Contains(out, "consulta") {
		t.Error("free text leaked into the metric labels")
	}
}
