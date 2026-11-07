package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

// The usage hook gets the tenant from the context and the model that served
// the call, which can be a fallback model (SPEC-CORE-69).
func TestCompleteReportsUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5",
			"content":[{"type":"text","text":"hola"}],"stop_reason":"end_turn",
			"usage":{"input_tokens":120,"output_tokens":30,"cache_read_input_tokens":40,"cache_creation_input_tokens":5}}`))
	}))
	defer srv.Close()

	c := NewAnthropicClient("test-key", "claude-opus-5-5", option.WithBaseURL(srv.URL), option.WithMaxRetries(0))
	var got []Usage
	c.OnUsage = func(_ context.Context, u Usage) { got = append(got, u) }

	out, err := c.Complete(WithTenant(context.Background(), "tenant_acme"), Request{User: "hola", Purpose: "answer"})
	if err != nil || out != "hola" {
		t.Fatalf("Complete: %q %v", out, err)
	}
	want := Usage{TenantID: "tenant_acme", Model: "claude-opus-5", Purpose: "answer", InputTokens: 120, OutputTokens: 30, CacheReadTokens: 40, CacheWriteTokens: 5}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("usage = %+v, want %+v", got, want)
	}
}
