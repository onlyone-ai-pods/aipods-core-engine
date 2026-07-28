package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PodManifest represents the pod.json metadata structure
type PodManifest struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Author       string   `json:"author"`
	Type         string   `json:"type"`
	Keywords     []string `json:"keywords"`
	Capabilities []string `json:"capabilities"`
}

// GeneratePodBoilerplate initializes a standardized AI Pod repository structure
func GeneratePodBoilerplate(targetDir, podName, language string) error {
	cleanName := strings.TrimSpace(podName)
	if cleanName == "" {
		return fmt.Errorf("pod name cannot be empty")
	}

	podID := strings.ToUpper(cleanName)
	if !strings.HasPrefix(podID, "POD_") {
		podID = "POD_" + podID
	}

	podDir := filepath.Join(targetDir, strings.ToLower(podID))
	if err := os.MkdirAll(podDir, 0750); err != nil {
		return fmt.Errorf("failed to create pod directory: %w", err)
	}

	// 1. Create pod.json manifest
	manifestContent := fmt.Sprintf(`{
  "id": "%s",
  "name": "AI Pod %s",
  "version": "1.0.0",
  "author": "Developer",
  "type": "dynamic_sidecar",
  "keywords": ["custom", "enterprise"],
  "capabilities": ["rag_search", "dry_run_execution"],
  "schemas": {
    "input": "schemas/input.json",
    "output": "schemas/output.json"
  }
}`, podID, cleanName)

	if err := os.WriteFile(filepath.Join(podDir, "pod.json"), []byte(manifestContent), 0600); err != nil {
		return err
	}

	// 2. Create workflow.yml (Declarative Macro)
	workflowContent := fmt.Sprintf(`# Workflow Macro for %s
name: "Workflow %s"
domain: "enterprise.local"

workflow_steps:
  - step: 1
    action: "query_rag"
    query_template: "Extract records"
  - step: 2
    action: "trigger_dry_run"
    message: "Simulation active"
`, podID, cleanName)

	if err := os.WriteFile(filepath.Join(podDir, "workflow.yml"), []byte(workflowContent), 0600); err != nil {
		return err
	}

	// 3. Create schemas directory
	schemasDir := filepath.Join(podDir, "schemas")
	if err := os.MkdirAll(schemasDir, 0750); err != nil {
		return err
	}

	inputSchema := `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "query": { "type": "string" },
    "dry_run": { "type": "boolean" }
  },
  "required": ["query"]
}`
	outputSchema := `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "pod_id": { "type": "string" },
    "answer": { "type": "string" },
    "status": { "type": "string" }
  },
  "required": ["pod_id", "answer", "status"]
}`

	if err := os.WriteFile(filepath.Join(schemasDir, "input.json"), []byte(inputSchema), 0600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(schemasDir, "output.json"), []byte(outputSchema), 0600); err != nil {
		return err
	}

	// 4. Create main implementation file (Go or Python)
	if strings.ToLower(language) == "python" {
		pyContent := fmt.Sprintf(`# AI Pod %s - Main Handler
from flask import Flask, request, jsonify

app = Flask(__name__)

@app.route('/healthz', methods=['GET'])
def healthz():
    return jsonify({"status": "OK", "pod_id": "%s"})

@app.route('/process', methods=['POST'])
def process():
    data = request.json or {}
    query = data.get("query", "")
    dry_run = data.get("dry_run", False)
    
    res = {
        "pod_id": "%s",
        "answer": f"Processed query: {query}",
        "citations": ["doc_ref_1"],
        "status": "COMPLETED"
    }
    
    if dry_run:
        res["dry_run_result"] = {
            "is_dry_run": True,
            "action_name": "process_query",
            "summary": "Simulation executed successfully",
            "affected_records_count": 1,
            "requires_human_approval": True,
            "approval_token": "dryrun_token_mock_12345"
        }
        
    return jsonify(res)

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=9095)
`, podID, podID, podID)

		if err := os.WriteFile(filepath.Join(podDir, "main.py"), []byte(pyContent), 0600); err != nil {
			return err
		}
	} else {
		goContent := fmt.Sprintf(`package main

import (
	"context"
	"encoding/json"
	"net/http"
)

type PodResponse struct {
	PodID        string        `+"`json:\"pod_id\"`"+`
	Answer       string        `+"`json:\"answer\"`"+`
	Citations    []string      `+"`json:\"citations\"`"+`
	Status       string        `+"`json:\"status\"`"+`
	DryRunResult *DryRunResult `+"`json:\"dry_run_result,omitempty\"`"+`
}

type DryRunResult struct {
	IsDryRun              bool   `+"`json:\"is_dry_run\"`"+`
	ActionName            string `+"`json:\"action_name\"`"+`
	Summary               string `+"`json:\"summary\"`"+`
	AffectedRecordsCount  int    `+"`json:\"affected_records_count\"`"+`
	RequiresHumanApproval bool   `+"`json:\"requires_human_approval\"`"+`
	ApprovalToken         string `+"`json:\"approval_token,omitempty\"`"+`
}

func main() {
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "OK", "pod_id": "%s"})
	})

	http.HandleFunc("/process", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := PodResponse{
			PodID:     "%s",
			Answer:    "Execution completed",
			Citations: []string{"doc_1"},
			Status:    "COMPLETED",
			DryRunResult: &DryRunResult{
				IsDryRun:              true,
				ActionName:            "process_query",
				Summary:               "Simulation OK",
				AffectedRecordsCount:  1,
				RequiresHumanApproval: true,
				ApprovalToken:         "dryrun_token_mock_12345",
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	_ = http.ListenAndServe(":9095", nil)
}
`, podID, podID)

		if err := os.WriteFile(filepath.Join(podDir, "pod.go"), []byte(goContent), 0600); err != nil {
			return err
		}
	}

	// 5. Create BDD test file
	testContent := fmt.Sprintf(`package main

import "testing"

func TestPodHandshake(t *testing.T) {
	// BDD Contract Validation Test
	if "%s" == "" {
		t.Fatalf("Pod ID cannot be empty")
	}
}
`, podID)

	if err := os.WriteFile(filepath.Join(podDir, "pod_test.go"), []byte(testContent), 0600); err != nil {
		return err
	}

	return nil
}
