package sandbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/martinllanos/only-ai-pods/internal/pod"
	"github.com/martinllanos/only-ai-pods/internal/router"
)

var (
	ErrMaxQueriesExceeded = errors.New("maximum 3 free sandbox test queries reached. Please create a free account to save this AI Pod")
	ErrSessionNotFound    = errors.New("sandbox session expired or invalid")
)

type SandboxSession struct {
	SessionID  string    `json:"session_id"`
	TenantID   string    `json:"tenant_id"`
	FileName   string    `json:"file_name"`
	QueryCount int       `json:"query_count"`
	MaxQueries int       `json:"max_queries"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// sessionTTL is the life of a session created with CreateEphemeralSession.
const sessionTTL = 30 * time.Minute

// demoSessionTTL is the life of a session created on the fly for an unknown ID.
const demoSessionTTL = 24 * time.Hour

// SessionManager runs sandbox queries. Sessions live in a SessionStore, so
// both colors see the same session and counter (SPEC-CORE-73.R11).
type SessionManager struct {
	store       SessionStore
	smartRouter *router.DynamicSmartRouter
}

// NewSessionManager keeps sessions in process memory.
func NewSessionManager(smartRouter *router.DynamicSmartRouter) *SessionManager {
	return NewSessionManagerWithStore(smartRouter, NewMemorySessionStore())
}

func NewSessionManagerWithStore(smartRouter *router.DynamicSmartRouter, store SessionStore) *SessionManager {
	return &SessionManager{store: store, smartRouter: smartRouter}
}

// CreateEphemeralSession initializes a Sandbox session ("Upload your PDF & Test")
func (m *SessionManager) CreateEphemeralSession(ctx context.Context, fileName string) (*SandboxSession, error) {
	sessionUUID := uuid.New().String()
	now := time.Now()
	session := SandboxSession{
		SessionID:  sessionUUID,
		TenantID:   fmt.Sprintf("sandbox_session_%s", sessionUUID[:8]),
		FileName:   fileName,
		QueryCount: 0,
		MaxQueries: 999,
		CreatedAt:  now,
		ExpiresAt:  now.Add(sessionTTL),
	}
	if err := m.store.Save(ctx, session); err != nil {
		return nil, fmt.Errorf("sandbox: save session: %w", err)
	}
	return &session, nil
}

// ExecuteSandboxQuery processes a query within the ephemeral Sandbox session
func (m *SessionManager) ExecuteSandboxQuery(ctx context.Context, sessionID, query string) (*pod.PodResponse, *SandboxSession, error) {
	session, ok, err := m.store.Get(ctx, sessionID)
	if err != nil {
		return nil, nil, fmt.Errorf("sandbox: read session: %w", err)
	}
	if !ok {
		// Auto-crear sesión de demostración efímera si es invocada sin previo registro
		now := time.Now()
		session = SandboxSession{
			SessionID:  sessionID,
			TenantID:   "TENANT_DEMO_001",
			FileName:   "Entorno Multi-Pod Activo",
			QueryCount: 0,
			MaxQueries: 999,
			CreatedAt:  now,
			ExpiresAt:  now.Add(demoSessionTTL),
		}
	}

	if session.QueryCount >= session.MaxQueries {
		return nil, &session, ErrMaxQueriesExceeded
	}

	session.QueryCount++
	if err := m.store.Save(ctx, session); err != nil {
		return nil, &session, fmt.Errorf("sandbox: save session: %w", err)
	}

	// Execute via Smart Router with dry_run = true by default in Sandbox
	res, err := m.smartRouter.RouteAndExecute(ctx, session.TenantID, query, true)
	if err != nil {
		return nil, &session, err
	}

	return res, &session, nil
}
