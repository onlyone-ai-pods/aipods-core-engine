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

// SlashCommand defines a command that can be triggered by typing '/' in the chat.
type SlashCommand struct {
	Command     string `json:"command"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Category    string `json:"category"`
	Example     string `json:"example"`
}

// SlashCommandProvider is an optional interface that AI Pods can implement
// to expose their available slash commands to the frontend.
type SlashCommandProvider interface {
	SlashCommands() []SlashCommand
}
