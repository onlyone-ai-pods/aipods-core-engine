package llm

import (
	"context"
	"strings"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

// DefaultModel is used when ANTHROPIC_MODEL is not set.
const DefaultModel = "claude-opus-5-5"

// AnthropicClient calls Claude through the official Go SDK.
type AnthropicClient struct {
	client anthropic.Client
	model  string
}

// NewAnthropicClient builds a client. An empty model uses DefaultModel.
func NewAnthropicClient(apiKey, model string) *AnthropicClient {
	if model == "" {
		model = DefaultModel
	}
	return &AnthropicClient{
		client: anthropic.NewClient(option.WithAPIKey(apiKey)),
		model:  model,
	}
}

func (a *AnthropicClient) Model() string { return a.model }

// Complete sends one user turn and returns the concatenated text answer.
// Server-side fallbacks are on, so a refusal is re-served by a fallback model
// inside the same call when possible.
func (a *AnthropicClient) Complete(ctx context.Context, req Request) (string, error) {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 16000
	}
	effort := anthropic.BetaOutputConfigEffort(req.Effort)
	if effort == "" {
		effort = anthropic.BetaOutputConfigEffort(EffortMedium)
	}

	params := anthropic.BetaMessageNewParams{
		Model:     a.model,
		MaxTokens: maxTokens,
		Betas:     []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01},
		Fallbacks: anthropic.BetaFallbacksParamOfDefault(),
		OutputConfig: anthropic.BetaOutputConfigParam{
			Effort: effort,
		},
		Messages: []anthropic.BetaMessageParam{
			anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(req.User)),
		},
	}
	if req.System != "" {
		params.System = []anthropic.BetaTextBlockParam{{Text: req.System}}
	}

	resp, err := a.client.Beta.Messages.New(ctx, params)
	if err != nil {
		return "", err
	}
	if resp.StopReason == anthropic.BetaStopReasonRefusal {
		return "", ErrRefused
	}

	var b strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.BetaTextBlock); ok {
			b.WriteString(t.Text)
		}
	}
	if b.Len() == 0 {
		return "", ErrNoText
	}
	return b.String(), nil
}
