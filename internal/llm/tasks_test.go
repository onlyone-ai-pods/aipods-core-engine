package llm

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

// scripted answers per model: an error, a text, or a block until ctx ends.
type scripted struct {
	mu      sync.Mutex
	errs    map[string]error
	block   map[string]bool
	calls   []string
	noRetry []bool
}

func (s *scripted) Model() string { return "base-model" }

func (s *scripted) Complete(ctx context.Context, req Request) (string, error) {
	s.mu.Lock()
	s.calls = append(s.calls, req.Model)
	s.noRetry = append(s.noRetry, req.NoRetry)
	block, err := s.block[req.Model], s.errs[req.Model]
	s.mu.Unlock()
	if block {
		<-ctx.Done()
		return "", ctx.Err()
	}
	if err != nil {
		return "", err
	}
	return "text from " + req.Model, nil
}

func status(code int) error { return &APIError{Status: code, Err: errors.New("api error")} }

func TestSPEC_CORE_74_R1_EmptyTaskModelUsesBaseModel(t *testing.T) {
	rt := &Runtime{}
	c := rt.Client(&scripted{}, TaskClassify, Models{Answer: "claude-opus-5-5"})
	if c.Model() != "base-model" {
		t.Fatalf("classify model = %q, want the base model", c.Model())
	}
	if got := rt.Client(&scripted{}, TaskAnswer, Models{Answer: "claude-opus-5-5"}).Model(); got != "claude-opus-5-5" {
		t.Fatalf("answer model = %q", got)
	}
}

func TestSPEC_CORE_74_R9_FallsBackOnSaturation(t *testing.T) {
	for _, code := range []int{429, 500, 502, 503, 504, 529} {
		base := &scripted{errs: map[string]error{"a": status(code)}}
		var hops [][2]string
		rt := &Runtime{OnFallback: func(_ Task, from, to string) { hops = append(hops, [2]string{from, to}) }}
		out, model, err := rt.Client(base, TaskAnswer, Models{Answer: "a", Fallback: []string{"b", "c"}}).
			CompleteWithModel(context.Background(), Request{User: "q"})
		if err != nil || model != "b" || out != "text from b" {
			t.Fatalf("status %d: out=%q model=%q err=%v", code, out, model, err)
		}
		if !reflect.DeepEqual(hops, [][2]string{{"a", "b"}}) {
			t.Fatalf("status %d: hops = %v", code, hops)
		}
	}
}

func TestSPEC_CORE_74_R9_DoesNotFallBackOnRejectedRequest(t *testing.T) {
	for name, err := range map[string]error{
		"400": status(400), "401": status(401), "403": status(403), "404": status(404),
		"refusal": ErrRefused, "no text": ErrNoText,
	} {
		base := &scripted{errs: map[string]error{"a": err}}
		_, _, got := (&Runtime{}).Client(base, TaskAnswer, Models{Answer: "a", Fallback: []string{"b"}}).
			CompleteWithModel(context.Background(), Request{})
		if got == nil || len(base.calls) != 1 {
			t.Errorf("%s: err=%v calls=%v, want one call and an error", name, got, base.calls)
		}
	}
}

func TestSPEC_CORE_74_R9_DoesNotFallBackWhenRequestIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	base := &scripted{block: map[string]bool{"a": true}}
	_, _, err := (&Runtime{}).Client(base, TaskAnswer, Models{Answer: "a", Fallback: []string{"b"}}).
		CompleteWithModel(ctx, Request{})
	if err == nil || len(base.calls) != 1 {
		t.Fatalf("err=%v calls=%v, want one call and an error", err, base.calls)
	}
}

func TestSPEC_CORE_74_R9_ExhaustedChainReturnsError(t *testing.T) {
	base := &scripted{errs: map[string]error{"a": status(529), "b": status(529)}}
	_, _, err := (&Runtime{}).Client(base, TaskExpand, Models{Expand: "a", Fallback: []string{"b"}}).
		CompleteWithModel(context.Background(), Request{})
	if err == nil || !reflect.DeepEqual(base.calls, []string{"a", "b"}) {
		t.Fatalf("err=%v calls=%v", err, base.calls)
	}
}

func TestSPEC_CORE_74_R10_ChainSkipsTaskModelAndDuplicates(t *testing.T) {
	c := (&Runtime{}).Client(&scripted{}, TaskAnswer, Models{Answer: "s", Fallback: []string{"s", "h", "h", ""}})
	if got := c.Chain(); !reflect.DeepEqual(got, []string{"s", "h"}) {
		t.Fatalf("chain = %v", got)
	}
}

func TestSPEC_CORE_74_R11_AttemptTimeoutThenRemainingBudget(t *testing.T) {
	base := &scripted{block: map[string]bool{"a": true, "b": true}}
	rt := &Runtime{
		Budget:         map[Task]time.Duration{TaskAnswer: 300 * time.Millisecond},
		AttemptTimeout: 60 * time.Millisecond,
	}
	var outcomes []string
	rt.OnRequest = func(_ Task, model, outcome string) { outcomes = append(outcomes, model+":"+outcome) }

	start := time.Now()
	_, _, err := rt.Client(base, TaskAnswer, Models{Answer: "a", Fallback: []string{"b"}}).
		CompleteWithModel(context.Background(), Request{})
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want deadline exceeded", err)
	}
	if !reflect.DeepEqual(base.calls, []string{"a", "b"}) {
		t.Fatalf("calls = %v: the first attempt must end at the attempt timeout", base.calls)
	}
	if elapsed < 250*time.Millisecond || elapsed > 600*time.Millisecond {
		t.Fatalf("task took %s, want about the 300ms budget", elapsed)
	}
	if !reflect.DeepEqual(outcomes, []string{"a:timeout", "b:timeout"}) {
		t.Fatalf("outcomes = %v", outcomes)
	}
}

func TestSPEC_CORE_74_R11_DefaultAttemptTimeoutIsHalfTheBudget(t *testing.T) {
	base := &scripted{block: map[string]bool{"a": true}}
	rt := &Runtime{Budget: map[Task]time.Duration{TaskClassify: 200 * time.Millisecond}}
	start := time.Now()
	_, model, err := rt.Client(base, TaskClassify, Models{Classify: "a", Fallback: []string{"b"}}).
		CompleteWithModel(context.Background(), Request{})
	if err != nil || model != "b" {
		t.Fatalf("model=%q err=%v", model, err)
	}
	if d := time.Since(start); d < 80*time.Millisecond || d > 180*time.Millisecond {
		t.Fatalf("first attempt lasted %s, want about 100ms", d)
	}
}

func TestSPEC_CORE_74_R12_NoSDKRetryExceptOnLastModel(t *testing.T) {
	base := &scripted{errs: map[string]error{"a": status(529), "b": status(529)}}
	_, _, _ = (&Runtime{}).Client(base, TaskAnswer, Models{Answer: "a", Fallback: []string{"b", "c"}}).
		CompleteWithModel(context.Background(), Request{})
	if !reflect.DeepEqual(base.noRetry, []bool{true, true, false}) {
		t.Fatalf("NoRetry per attempt = %v", base.noRetry)
	}

	single := &scripted{}
	_, _ = (&Runtime{}).Client(single, TaskAnswer, Models{Answer: "a"}).Complete(context.Background(), Request{})
	if !reflect.DeepEqual(single.noRetry, []bool{false}) {
		t.Fatalf("without a chain the SDK retries stay on: %v", single.noRetry)
	}
}

func TestParseModelList(t *testing.T) {
	got := ParseModelList(" a , b,,a ,c ")
	if !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("got %v", got)
	}
	if ParseModelList("  ") != nil {
		t.Fatal("blank list must be nil")
	}
}
