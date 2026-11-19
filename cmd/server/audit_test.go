package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/martinllanos/only-ai-pods/internal/auth"
	"github.com/martinllanos/only-ai-pods/internal/store"
)

// ctxCheckingStore fails AppendAudit when the context is already cancelled,
// like Postgres does.
type ctxCheckingStore struct {
	store.Store
	events []store.AuditEvent
}

func (s *ctxCheckingStore) AppendAudit(ctx context.Context, e store.AuditEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.events = append(s.events, e)
	return nil
}

func TestSPEC_CORE_73_R4_AuditSurvivesCancelledRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := &ctxCheckingStore{Store: store.NewMemory()}
	a := auditor{st: st, guard: &auth.Guard{}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // client gone or drain timeout reached
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/vault/reveal", nil).WithContext(ctx)

	a.recordAs(c, "admin@aipods.ar", "GLOBAL", "vault.reveal", "KEY", "success")
	if len(st.events) != 1 || st.events[0].Action != "vault.reveal" {
		t.Fatalf("audit events = %+v, want the vault.reveal event", st.events)
	}
}
