// Command cmmi-report writes the CMMI process metrics of a git checkout to a
// JSON file (SPEC-CORE-68). The Dockerfile runs it while building the image.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/martinllanos/only-ai-pods/internal/cmmi"
)

func main() {
	repo := flag.String("repo", ".", "git checkout to read")
	out := flag.String("out", "cmmi.json", "output file")
	flag.Parse()

	commits, err := readCommits(*repo)
	if err != nil {
		log.Fatalf("cmmi-report: %v", err)
	}
	lines, err := cmmi.CountGoLines(*repo)
	if err != nil {
		log.Fatalf("cmmi-report: count lines: %v", err)
	}
	r := cmmi.Compute(commits, lines, time.Now())
	r.Commit = strings.TrimSpace(git(*repo, "rev-parse", "--short", "HEAD"))
	r.Shallow = strings.TrimSpace(git(*repo, "rev-parse", "--is-shallow-repository")) == "true"

	b, _ := json.MarshalIndent(r, "", "  ")
	if err := os.WriteFile(*out, b, 0o644); err != nil {
		log.Fatalf("cmmi-report: %v", err)
	}
	fmt.Printf("cmmi-report: %d commits, STI %.2f, %d fixes / %.2f KLOC, %d specs\n",
		r.CommitsTotal, r.TraceabilityIndex, r.FixCommits, r.GoKLOC, len(r.Specs))
}

func readCommits(repo string) ([]cmmi.Commit, error) {
	cmd := exec.Command("git", "-C", repo, "log", "--no-merges", "--format=%H%x09%cI%x09%s")
	outb, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	var commits []cmmi.Commit
	for _, line := range strings.Split(strings.TrimSpace(string(outb)), "\n") {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		d, err := time.Parse(time.RFC3339, parts[1])
		if err != nil {
			continue
		}
		commits = append(commits, cmmi.Commit{Hash: parts[0], Date: d, Subject: parts[2]})
	}
	return commits, nil
}

func git(repo string, args ...string) string {
	b, _ := exec.Command("git", append([]string{"-C", repo}, args...)...).Output()
	return string(b)
}
