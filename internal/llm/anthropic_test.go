package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

func TestSPEC_CORE_73_R3_CompleteHonoursTimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select { // never answers on its own
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer srv.Close()
	defer close(release) // runs first, so Close does not wait on the handler

	c := NewAnthropicClient("test-key", "", option.WithBaseURL(srv.URL), option.WithMaxRetries(0))
	if c.Timeout() != DefaultTimeout {
		t.Fatalf("default timeout = %s, want %s", c.Timeout(), DefaultTimeout)
	}
	c.SetTimeout(200 * time.Millisecond)
	c.SetTimeout(0) // ignored

	begin := time.Now()
	_, err := c.Complete(context.Background(), Request{User: "hola"})
	if err == nil {
		t.Fatal("Complete succeeded against a server that never answers")
	}
	if elapsed := time.Since(begin); elapsed > 3*time.Second {
		t.Fatalf("Complete took %s with a 200ms timeout", elapsed)
	}
}
