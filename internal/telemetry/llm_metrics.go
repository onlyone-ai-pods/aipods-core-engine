package telemetry

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// LLM requests and fallbacks by task and model (SPEC-CORE-74.R15).

// otherLabel replaces any value outside the known sets, so the metric
// cardinality stays bounded.
const otherLabel = "other"

var (
	llmTasks    = map[string]bool{"classify": true, "expand": true, "answer": true}
	llmOutcomes = map[string]bool{"ok": true, "error": true, "timeout": true, "refused": true}
)

type llmMetrics struct {
	mu        sync.Mutex
	models    map[string]bool
	requests  map[[3]string]int64 // task, model, outcome
	fallbacks map[[3]string]int64 // task, from, to
}

func newLLMMetrics() *llmMetrics {
	return &llmMetrics{models: map[string]bool{}, requests: map[[3]string]int64{}, fallbacks: map[[3]string]int64{}}
}

func bounded(known map[string]bool, v string) string {
	if known[v] {
		return v
	}
	return otherLabel
}

// SetLLMModels sets the model names accepted as label values: the allowed
// list of the platform. Any other model is reported as "other".
func (m *MetricsCollector) SetLLMModels(models []string) {
	m.llm.mu.Lock()
	defer m.llm.mu.Unlock()
	m.llm.models = make(map[string]bool, len(models))
	for _, name := range models {
		m.llm.models[name] = true
	}
}

// RecordLLMRequest counts one call to a model for a task.
func (m *MetricsCollector) RecordLLMRequest(task, model, outcome string) {
	m.llm.mu.Lock()
	defer m.llm.mu.Unlock()
	m.llm.requests[[3]string{bounded(llmTasks, task), bounded(m.llm.models, model), bounded(llmOutcomes, outcome)}]++
}

// RecordLLMFallback counts one hop of the fallback chain.
func (m *MetricsCollector) RecordLLMFallback(task, from, to string) {
	m.llm.mu.Lock()
	defer m.llm.mu.Unlock()
	m.llm.fallbacks[[3]string{bounded(llmTasks, task), bounded(m.llm.models, from), bounded(m.llm.models, to)}]++
}

// prometheus writes both metric families, sorted for a stable scrape.
func (l *llmMetrics) prometheus(b *strings.Builder) {
	l.mu.Lock()
	defer l.mu.Unlock()
	write := func(name, help string, labels [3]string, data map[[3]string]int64) {
		fmt.Fprintf(b, "# HELP %s %s\n# TYPE %s counter\n", name, help, name)
		keys := make([][3]string, 0, len(data))
		for k := range data {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return strings.Join(keys[i][:], "\x00") < strings.Join(keys[j][:], "\x00") })
		for _, k := range keys {
			fmt.Fprintf(b, "%s{%s=%q,%s=%q,%s=%q} %d\n", name, labels[0], k[0], labels[1], k[1], labels[2], k[2], data[k])
		}
		b.WriteString("\n")
	}
	write("aipods_llm_requests_total", "LLM calls by task, model and outcome.", [3]string{"task", "model", "outcome"}, l.requests)
	write("aipods_llm_fallback_total", "Hops of the LLM fallback chain by task.", [3]string{"task", "from", "to"}, l.fallbacks)
}
