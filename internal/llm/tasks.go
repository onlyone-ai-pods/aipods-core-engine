package llm

import (
	"context"
	"errors"
	"log"
	"net"
	"strings"
	"time"

	"github.com/martinllanos/only-ai-pods/internal/tenant"
)

// Model per task and fallback chain (SPEC-CORE-74).

// Task is one of the three LLM calls a chat query makes.
type Task string

const (
	TaskClassify Task = "classify"
	TaskExpand   Task = "expand"
	TaskAnswer   Task = "answer"
)

// Tasks lists every task, in the order a query runs them.
var Tasks = []Task{TaskClassify, TaskExpand, TaskAnswer}

// MaxFallbackModels caps the fallback chain (SPEC-CORE-74.R10).
const MaxFallbackModels = 3

// Request outcomes reported to Runtime.OnRequest (SPEC-CORE-74.R15).
const (
	OutcomeOK      = "ok"
	OutcomeError   = "error"
	OutcomeTimeout = "timeout"
	OutcomeRefused = "refused"
)

// Models is the model of each task plus the fallback chain. An empty task
// model means "the base client's model".
type Models struct {
	Classify string
	Expand   string
	Answer   string
	Fallback []string
}

// For returns the model configured for task, or "".
func (m Models) For(task Task) string {
	switch task {
	case TaskClassify:
		return m.Classify
	case TaskExpand:
		return m.Expand
	case TaskAnswer:
		return m.Answer
	}
	return ""
}

// ParseModelList splits a comma-separated list, trimming spaces and dropping
// empty items and duplicates, keeping the order.
func ParseModelList(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

// Runtime is the process-wide configuration of the task clients.
type Runtime struct {
	// Defaults are the environment-level models (SPEC-CORE-74.R1).
	Defaults Models
	// Budget bounds a whole task, fallback chain included (R11). A task
	// without an entry has no budget of its own.
	Budget map[Task]time.Duration
	// AttemptTimeout bounds one attempt when a chain exists; zero means half
	// the task budget.
	AttemptTimeout time.Duration
	// OnRequest and OnFallback feed the metrics (R15). Both are optional.
	OnRequest  func(task Task, model, outcome string)
	OnFallback func(task Task, from, to string)
}

// Client binds base to one task with the given models.
func (rt *Runtime) Client(base Client, task Task, m Models) *TaskClient {
	primary := m.For(task)
	if primary == "" {
		primary = base.Model()
	}
	chain := []string{primary}
	for _, f := range m.Fallback {
		dup := false
		for _, c := range chain {
			if c == f {
				dup = true
				break
			}
		}
		if !dup && f != "" {
			chain = append(chain, f)
		}
	}
	return &TaskClient{base: base, task: task, chain: chain, rt: rt}
}

// TaskClient is a Client for one task: it runs the request on the task's
// model and, when that model is saturated, on the next models of the chain.
type TaskClient struct {
	base  Client
	task  Task
	chain []string
	rt    *Runtime
}

// Model is the task's configured model (the first of the chain).
func (t *TaskClient) Model() string { return t.chain[0] }

// Chain returns the models this client may use, in order.
func (t *TaskClient) Chain() []string { return append([]string(nil), t.chain...) }

// Complete implements Client.
func (t *TaskClient) Complete(ctx context.Context, req Request) (string, error) {
	out, _, err := t.CompleteWithModel(ctx, req)
	return out, err
}

// CompleteWithModel also reports which model produced the text (R13).
func (t *TaskClient) CompleteWithModel(ctx context.Context, req Request) (string, string, error) {
	parent := ctx
	if budget := t.rt.Budget[t.task]; budget > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, budget)
		defer cancel()
	}

	var lastErr error
	for i, model := range t.chain {
		last := i == len(t.chain)-1
		attemptCtx, cancel := t.attemptContext(ctx, last)
		attempt := req
		attempt.Model = model
		// The next attempt is the next model, not the same saturated one (R12).
		attempt.NoRetry = !last
		out, err := t.base.Complete(attemptCtx, attempt)
		cancel()

		outcome := classifyOutcome(err)
		if t.rt.OnRequest != nil {
			t.rt.OnRequest(t.task, model, outcome)
		}
		if err == nil {
			return out, model, nil
		}
		lastErr = err
		// The client went away or the core is draining: stop (R9).
		if parent.Err() != nil || last || !fallbackable(err) || ctx.Err() != nil {
			break
		}
		next := t.chain[i+1]
		if t.rt.OnFallback != nil {
			t.rt.OnFallback(t.task, model, next)
		}
		tenantID, _ := tenant.FromContext(parent)
		log.Printf("[llm] fallback task=%s from=%s to=%s reason=%s tenant=%s", t.task, model, next, outcome, tenantID)
	}
	return "", "", lastErr
}

// attemptContext bounds one attempt: the last one gets what is left of the
// task budget, the others at most AttemptTimeout (R11).
func (t *TaskClient) attemptContext(ctx context.Context, last bool) (context.Context, context.CancelFunc) {
	if last || len(t.chain) == 1 {
		return context.WithCancel(ctx)
	}
	d := t.rt.AttemptTimeout
	if d <= 0 {
		d = t.rt.Budget[t.task] / 2
	}
	if d <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, d)
}

// classifyOutcome maps an error to a metric label.
func classifyOutcome(err error) string {
	switch {
	case err == nil:
		return OutcomeOK
	case errors.Is(err, ErrRefused):
		return OutcomeRefused
	case errors.Is(err, context.DeadlineExceeded):
		return OutcomeTimeout
	}
	var nerr net.Error
	if errors.As(err, &nerr) && nerr.Timeout() {
		return OutcomeTimeout
	}
	return OutcomeError
}

// StatusCoder is implemented by provider errors that carry an HTTP status.
type StatusCoder interface{ HTTPStatus() int }

// fallbackable reports whether another model may answer after err (R9):
// saturation, server errors, timeouts and connection errors, but not a
// rejected request, a refusal or an empty answer.
func fallbackable(err error) bool {
	if errors.Is(err, ErrRefused) || errors.Is(err, ErrNoText) || errors.Is(err, context.Canceled) {
		return false
	}
	var sc StatusCoder
	if errors.As(err, &sc) {
		switch sc.HTTPStatus() {
		case 429, 500, 502, 503, 504, 529:
			return true
		}
		return false
	}
	return true
}
