// Package cmmi computes the process metrics shown in /metrics from the git
// history of the repository (SPEC-CORE-68). The report is produced when the
// image is built and read by the server at start-up.
package cmmi

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Commit is the part of a git commit the metrics need.
type Commit struct {
	Hash    string
	Date    time.Time
	Subject string
}

// SpecLeadTime is how long a spec took, from its first to its last commit.
type SpecLeadTime struct {
	Spec    string  `json:"spec"`
	Commits int     `json:"commits"`
	First   string  `json:"first"`
	Last    string  `json:"last"`
	Hours   float64 `json:"hours"`
}

// Report holds the computed metrics. Fields are zero when there is no data.
type Report struct {
	GeneratedAt string `json:"generated_at"`
	Commit      string `json:"commit"`
	Shallow     bool   `json:"shallow"`

	CommitsTotal         int     `json:"commits_total"`
	CommitsWithReference int     `json:"commits_with_reference"`
	TraceabilityIndex    float64 `json:"spec_traceability_index"`

	FixCommits    int     `json:"fix_commits"`
	GoKLOC        float64 `json:"go_kloc"`
	DefectDensity float64 `json:"defect_density_per_kloc"`

	Specs              []SpecLeadTime `json:"specs"`
	AvgSpecLeadTimeHrs float64        `json:"avg_spec_lead_time_hours"`
}

var (
	// A reference the pre-commit hook accepts: RFC-NNNN, SPEC-NNNN, SPEC-XXX-NN or issue #NN.
	refRe  = regexp.MustCompile(`\b(RFC-\d{4}|SPEC-\d{4}|SPEC-[A-Z]+-\d+)\b|#\d+`)
	specRe = regexp.MustCompile(`\bSPEC-(?:\d{4}|[A-Z]+-\d+)\b`)
	// GitHub appends "(#N)" to squash merges: that is the PR number, not a reference.
	prSuffixRe = regexp.MustCompile(`\s*\(#\d+\)\s*$`)
	fixRe      = regexp.MustCompile(`(?i)^\s*(\[fix\]|fix(\(.*\))?:)`)
)

// Compute builds the report from the commits and the size of the Go code.
func Compute(commits []Commit, goLines int, now time.Time) Report {
	r := Report{GeneratedAt: now.UTC().Format(time.RFC3339), CommitsTotal: len(commits), GoKLOC: round(float64(goLines)/1000, 2)}
	type span struct {
		n           int
		first, last time.Time
	}
	specs := map[string]*span{}
	for _, c := range commits {
		subject := prSuffixRe.ReplaceAllString(c.Subject, "")
		if refRe.MatchString(subject) {
			r.CommitsWithReference++
		}
		if fixRe.MatchString(subject) {
			r.FixCommits++
		}
		seen := map[string]bool{}
		for _, id := range specRe.FindAllString(subject, -1) {
			if seen[id] {
				continue
			}
			seen[id] = true
			s := specs[id]
			if s == nil {
				s = &span{first: c.Date, last: c.Date}
				specs[id] = s
			}
			s.n++
			if c.Date.Before(s.first) {
				s.first = c.Date
			}
			if c.Date.After(s.last) {
				s.last = c.Date
			}
		}
	}
	if r.CommitsTotal > 0 {
		r.TraceabilityIndex = round(float64(r.CommitsWithReference)/float64(r.CommitsTotal), 4)
	}
	if r.GoKLOC > 0 {
		r.DefectDensity = round(float64(r.FixCommits)/r.GoKLOC, 4)
	}
	// Lead time only means something for specs delivered in more than one commit.
	var total float64
	for id, s := range specs {
		if s.n < 2 {
			continue
		}
		h := round(s.last.Sub(s.first).Hours(), 2)
		r.Specs = append(r.Specs, SpecLeadTime{Spec: id, Commits: s.n, First: s.first.UTC().Format(time.RFC3339), Last: s.last.UTC().Format(time.RFC3339), Hours: h})
		total += h
	}
	sort.Slice(r.Specs, func(i, j int) bool { return r.Specs[i].Spec < r.Specs[j].Spec })
	if len(r.Specs) > 0 {
		r.AvgSpecLeadTimeHrs = round(total/float64(len(r.Specs)), 2)
	}
	return r
}

// Load reads a report written by cmd/cmmi-report.
func Load(path string) (*Report, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Report
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// CountGoLines counts lines of non-test Go source under root, skipping
// vendor, testdata and hidden directories.
func CountGoLines(root string) (int, error) {
	n := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != root && (strings.HasPrefix(name, ".") || name == "vendor" || name == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		n += bytes.Count(b, []byte("\n"))
		return nil
	})
	return n, err
}

func round(v float64, places int) float64 {
	p := 1.0
	for i := 0; i < places; i++ {
		p *= 10
	}
	if v < 0 {
		return -float64(int64(-v*p+0.5)) / p
	}
	return float64(int64(v*p+0.5)) / p
}
