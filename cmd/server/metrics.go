package main

import (
	"log"

	"github.com/martinllanos/only-ai-pods/internal/cmmi"
)

// loadCMMIReport reads the report that cmd/cmmi-report wrote while building
// the image (SPEC-CORE-68). Without it, /metrics leaves the CMMI gauges out
// instead of showing invented numbers.
func loadCMMIReport(getenv func(string) string) *cmmi.Report {
	path := getenv("CMMI_REPORT_FILE")
	if path == "" {
		path = "cmmi.json"
	}
	r, err := cmmi.Load(path)
	if err != nil {
		log.Printf("[metrics] no CMMI report at %s: %v (CMMI metrics disabled)", path, err)
		return nil
	}
	log.Printf("[metrics] CMMI report from commit %s: %d commits, STI %.2f", r.Commit, r.CommitsTotal, r.TraceabilityIndex)
	return r
}
