package llm

import (
	"context"
	"errors"
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

// serverSideFallback lists the models that accept the server-side fallback
// for refusals. Models not listed get the conservative request without it:
// claude-haiku-5-5 has no server-side fallback (SPEC-CORE-74.R4).
var serverSideFallback = map[string]bool{
	"claude-fable-5-2":  true,
	"claude-fable-5-1":  true,
	"claude-opus-5-5":   true,
	"claude-opus-5":     true,
	"claude-sonnet-5-5": true,
}

// APIError is a provider error with its HTTP status, so the fallback chain
// can tell saturation from a rejected request (SPEC-CORE-74.R9).
type APIError struct {
	Status int
	Err    error
}

func (e *APIError) Error() string   { return e.Err.Error() }
func (e *APIError) Unwrap() error   { return e.Err }
func (e *APIError) HTTPStatus() int { return e.Status }

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

	model := req.Model
	if model == "" {
		model = a.model
	}
	params := anthropic.BetaMessageNewParams{
		Model:     model,
		MaxTokens: maxTokens,
		OutputConfig: anthropic.BetaOutputConfigParam{
			Effort: effort,
		},
		Messages: []anthropic.BetaMessageParam{
			anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(req.User)),
		},
	}
	// Only some models re-serve a refusal on the server side (SPEC-CORE-74.R4).
	if serverSideFallback[model] {
		params.Betas = []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01}
		params.Fallbacks = anthropic.BetaFallbacksParamOfDefault()
	}
	if req.System != "" {
		params.System = []anthropic.BetaTextBlockParam{{Text: req.System}}
	}

	var opts []option.RequestOption
	if req.NoRetry {
		opts = append(opts, option.WithMaxRetries(0))
	}
	resp, err := a.client.Beta.Messages.New(ctx, params, opts...)
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) {
			return "", &APIError{Status: apiErr.StatusCode, Err: err}
		}
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
