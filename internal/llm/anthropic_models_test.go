package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

const okMessage = `{"id":"msg_1","type":"message","role":"assistant","model":"m","content":[{"type":"text","text":"hola"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`

// apiServer records each request body and answers with the status scripted
// for the request's model (200 by default).
type apiServer struct {
	mu     sync.Mutex
	bodies []map[string]any
	betas  []string
	status map[string]int
}

func (a *apiServer) start(t *testing.T) *AnthropicClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		model, _ := body["model"].(string)
		a.mu.Lock()
		a.bodies = append(a.bodies, body)
		a.betas = append(a.betas, r.Header.Get("anthropic-beta"))
		code := a.status[model]
		a.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if code != 0 {
			w.Header().Set("x-should-retry", "true")
			w.WriteHeader(code)
			_, _ = w.Write([]byte(`{"type":"error","error":{"type":"overloaded_error","message":"overloaded"}}`))
			return
		}
		_, _ = w.Write([]byte(okMessage))
	}))
	t.Cleanup(srv.Close)
	return NewAnthropicClient("test-key", "claude-opus-5-5", option.WithBaseURL(srv.URL))
}

func (a *apiServer) countFor(model string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	n := 0
	for _, b := range a.bodies {
		if b["model"] == model {
			n++
		}
	}
	return n
}

func TestSPEC_CORE_74_R3_RequestModelOverridesClientModel(t *testing.T) {
	api := &apiServer{}
	c := api.start(t)
	if _, err := c.Complete(context.Background(), Request{User: "hola"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Complete(context.Background(), Request{User: "hola", Model: "claude-haiku-5-5"}); err != nil {
		t.Fatal(err)
	}
	if api.bodies[0]["model"] != "claude-opus-5-5" || api.bodies[1]["model"] != "claude-haiku-5-5" {
		t.Fatalf("models sent: %v, %v", api.bodies[0]["model"], api.bodies[1]["model"])
	}
}

func TestSPEC_CORE_74_R4_ParametersDependOnTheModel(t *testing.T) {
	api := &apiServer{}
	c := api.start(t)
	for _, model := range []string{"claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5", "some-future-model"} {
		if _, err := c.Complete(context.Background(), Request{User: "hola", Model: model, Effort: EffortLow}); err != nil {
			t.Fatalf("%s: %v", model, err)
		}
	}
	for i, wantFallbacks := range []bool{true, true, false, false} {
		body := api.bodies[i]
		if _, has := body["fallbacks"]; has != wantFallbacks {
			t.Errorf("%v: fallbacks present = %t, want %t", body["model"], has, wantFallbacks)
		}
		if hasBeta := api.betas[i] != ""; hasBeta != wantFallbacks {
			t.Errorf("%v: beta header %q, want present = %t", body["model"], api.betas[i], wantFallbacks)
		}
		cfg, _ := body["output_config"].(map[string]any)
		if cfg["effort"] != "low" {
			t.Errorf("%v: effort = %v, want it always explicit", body["model"], cfg["effort"])
		}
	}
}

func TestSPEC_CORE_74_R9_APIErrorCarriesTheStatus(t *testing.T) {
	api := &apiServer{status: map[string]int{"claude-opus-5-5": 529}}
	c := api.start(t)
	_, err := c.Complete(context.Background(), Request{User: "hola", NoRetry: true})
	var sc StatusCoder
	if !errors.As(err, &sc) || sc.HTTPStatus() != 529 {
		t.Fatalf("err = %v, want a status 529", err)
	}
}

func TestSPEC_CORE_74_R12_ChainTurnsOffSDKRetriesUntilTheLastModel(t *testing.T) {
	api := &apiServer{status: map[string]int{"claude-opus-5-5": 529, "claude-haiku-5-5": 529}}
	c := api.start(t)
	rt := &Runtime{}
	_, _, err := rt.Client(c, TaskAnswer, Models{Fallback: []string{"claude-haiku-5-5"}}).
		CompleteWithModel(context.Background(), Request{User: "hola"})
	if err == nil {
		t.Fatal("want an error when every model is saturated")
	}
	if n := api.countFor("claude-opus-5-5"); n != 1 {
		t.Errorf("first model got %d requests, want 1 (no SDK retry)", n)
	}
	if n := api.countFor("claude-haiku-5-5"); n != 3 {
		t.Errorf("last model got %d requests, want 3 (SDK retries on)", n)
	}
}

func TestSPEC_CORE_74_R12_WithoutChainSDKRetriesStayOn(t *testing.T) {
	api := &apiServer{status: map[string]int{"claude-opus-5-5": 529}}
	c := api.start(t)
	_, _ = (&Runtime{}).Client(c, TaskAnswer, Models{}).Complete(context.Background(), Request{User: "hola"})
	if n := api.countFor("claude-opus-5-5"); n != 3 {
		t.Errorf("got %d requests, want 3", n)
	}
}
