package sandbox

import (
	"context"
	"errors"
	"fmt"
	"sync"
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
	SessionID   string    `json:"session_id"`
	TenantID    string    `json:"tenant_id"`
	FileName    string    `json:"file_name"`
	QueryCount  int       `json:"query_count"`
	MaxQueries  int       `json:"max_queries"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type SessionManager struct {
	mu          sync.RWMutex
	sessions    map[string]*SandboxSession
	smartRouter *router.DynamicSmartRouter
}

func NewSessionManager(smartRouter *router.DynamicSmartRouter) *SessionManager {
	return &SessionManager{
		sessions:    make(map[string]*SandboxSession),
		smartRouter: smartRouter,
	}
}

// CreateEphemeralSession initializes a Sandbox session ("Upload your PDF & Test")
func (m *SessionManager) CreateEphemeralSession(fileName string) *SandboxSession {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessionUUID := uuid.New().String()
	tenantID := fmt.Sprintf("sandbox_session_%s", sessionUUID[:8])

	session := &SandboxSession{
		SessionID:  sessionUUID,
		TenantID:   tenantID,
		FileName:   fileName,
		QueryCount: 0,
		MaxQueries: 3,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(30 * time.Minute),
	}

	m.sessions[sessionUUID] = session
	return session
}

// ExecuteSandboxQuery processes a query within the ephemeral Sandbox session
func (m *SessionManager) ExecuteSandboxQuery(ctx context.Context, sessionID, query string) (*pod.PodResponse, *SandboxSession, error) {
	m.mu.Lock()
	session, ok := m.sessions[sessionID]
	if !ok || time.Now().After(session.ExpiresAt) {
		m.mu.Unlock()
		return nil, nil, ErrSessionNotFound
	}

	if session.QueryCount >= session.MaxQueries {
		m.mu.Unlock()
		return nil, session, ErrMaxQueriesExceeded
	}

	session.QueryCount++
	m.mu.Unlock()

	// Execute via Smart Router with dry_run = true by default in Sandbox
	res, err := m.smartRouter.RouteAndExecute(ctx, session.TenantID, query, true)
	if err != nil {
		return nil, session, err
	}

	return res, session, nil
}
