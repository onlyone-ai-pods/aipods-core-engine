package pod

import (
	"context"
)

type DryRunResult struct {
	IsDryRun              bool        `json:"is_dry_run"`
	ActionName            string      `json:"action_name"`
	Summary               string      `json:"summary"`
	AffectedRecordsCount  int         `json:"affected_records_count"`
	GeneratedCommand      string      `json:"generated_command,omitempty"`
	RequiresHumanApproval bool        `json:"requires_human_approval"`
	ApprovalToken         string      `json:"approval_token,omitempty"`
}

type PodResponse struct {
	PodID        string       `json:"pod_id"`
	Answer       string       `json:"answer"`
	Citations    []string     `json:"citations"`
	DryRunResult *DryRunResult `json:"dry_run_result,omitempty"`
	Status       string       `json:"status"`
}

type BaseAIPod interface {
	ID() string
	Name() string
	ProcessQuery(ctx context.Context, tenantID string, query string, dryRun bool) (*PodResponse, error)
}
