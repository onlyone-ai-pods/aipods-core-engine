package llm

import (
	"context"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

// DefaultModel is used when ANTHROPIC_MODEL is not set.
const DefaultModel = "claude-opus-5-5"

// DefaultTimeout bounds one LLM call, so a shutdown drain has a known maximum
// (SPEC-CORE-73.R3). LLM_TIMEOUT overrides it.
const DefaultTimeout = 45 * time.Second

// AnthropicClient calls Claude through the official Go SDK.
type AnthropicClient struct {
	client  anthropic.Client
	model   string
	timeout time.Duration
}

// NewAnthropicClient builds a client. An empty model uses DefaultModel.
// Extra SDK options (base URL, HTTP client) are for tests.
func NewAnthropicClient(apiKey, model string, opts ...option.RequestOption) *AnthropicClient {
	if model == "" {
		model = DefaultModel
	}
	return &AnthropicClient{
		client:  anthropic.NewClient(append([]option.RequestOption{option.WithAPIKey(apiKey)}, opts...)...),
		model:   model,
		timeout: DefaultTimeout,
	}
}

// SetTimeout changes the per-call timeout; zero or negative keeps the current one.
func (a *AnthropicClient) SetTimeout(d time.Duration) {
	if d > 0 {
		a.timeout = d
	}
}

// Timeout is the per-call bound applied by Complete.
func (a *AnthropicClient) Timeout() time.Duration { return a.timeout }

func (a *AnthropicClient) Model() string { return a.model }

// Complete sends one user turn and returns the concatenated text answer.
// Server-side fallbacks are on, so a refusal is re-served by a fallback model
// inside the same call when possible.
func (a *AnthropicClient) Complete(ctx context.Context, req Request) (string, error) {
	if a.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.timeout)
		defer cancel()
	}
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
