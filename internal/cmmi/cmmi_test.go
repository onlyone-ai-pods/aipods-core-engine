package cmmi

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func at(s string) time.Time { t, _ := time.Parse(time.RFC3339, s); return t }

func TestCompute(t *testing.T) {
	commits := []Commit{
		{Date: at("2026-11-07T05:00:00Z"), Subject: "[FEAT] revoke sessions (SPEC-CORE-67) (#7)"},
		{Date: at("2026-11-06T21:00:00Z"), Subject: "[FEAT] auth and postgres (SPEC-CORE-67) (#2)"},
		{Date: at("2026-11-07T02:00:00Z"), Subject: "[FIX] real PDF text (SPEC-CORE-66) (#5)"},
		{Date: at("2026-11-06T10:00:00Z"), Subject: "fix: typo in readme (#9)"}, // only the PR suffix: no reference
		{Date: at("2026-11-06T09:00:00Z"), Subject: "chore: closes #12"},
	}
	r := Compute(commits, 2500, at("2026-11-07T06:00:00Z"))
	if r.CommitsTotal != 5 || r.CommitsWithReference != 4 || r.TraceabilityIndex != 0.8 {
		t.Errorf("traceability: %+v", r)
	}
	if r.FixCommits != 2 || r.GoKLOC != 2.5 || r.DefectDensity != 0.8 {
		t.Errorf("defects: fixes=%d kloc=%v density=%v", r.FixCommits, r.GoKLOC, r.DefectDensity)
	}
	// SPEC-CORE-66 has one commit, so only SPEC-CORE-67 has a lead time: 21:00 -> 05:00 = 8 h.
	if len(r.Specs) != 1 || r.Specs[0].Spec != "SPEC-CORE-67" || r.Specs[0].Hours != 8 || r.AvgSpecLeadTimeHrs != 8 {
		t.Errorf("lead time: %+v avg=%v", r.Specs, r.AvgSpecLeadTimeHrs)
	}
}

func TestComputeEmpty(t *testing.T) {
	r := Compute(nil, 0, time.Now())
	if r.TraceabilityIndex != 0 || r.DefectDensity != 0 || r.AvgSpecLeadTimeHrs != 0 {
		t.Errorf("empty history must give zeros: %+v", r)
	}
}

func TestCountGoLines(t *testing.T) {
	dir := t.TempDir()
	write := func(p, s string) {
		_ = os.MkdirAll(filepath.Dir(filepath.Join(dir, p)), 0o755)
		_ = os.WriteFile(filepath.Join(dir, p), []byte(s), 0o644)
	}
	write("a.go", "package a\n\nfunc A() {}\n")
	write("a_test.go", "package a\n")
	write("vendor/x.go", "package x\n")
	write("internal/testdata/y.go", "package y\n")
	write("README.md", "hello\n")
	if n, err := CountGoLines(dir); err != nil || n != 3 {
		t.Errorf("CountGoLines = %d, %v; want 3", n, err)
	}
}
