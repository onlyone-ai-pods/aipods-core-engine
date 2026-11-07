package store

import (
	"context"
	"github.com/martinllanos/only-ai-pods/internal/tenant"
	"sort"
	"strings"
	"sync"
	"time"
)

// Memory is a non-persistent Store for development and tests.
type Memory struct {
	mu        sync.RWMutex
	tenants   map[string]string
	users     map[string]User // by lowercased email
	approvals map[string]Approval
	secrets   map[string]Secret // by tenant/key
	audit     []AuditEvent
	ragStrat  map[string]string // by tenant id; missing = default
}

func NewMemory() *Memory {
	return &Memory{
		tenants:   map[string]string{"GLOBAL": "Plataforma AI Pods"},
		users:     map[string]User{},
		approvals: map[string]Approval{},
		secrets:   map[string]Secret{},
		ragStrat:  map[string]string{},
	}
}

func (m *Memory) Name() string               { return "memory" }
func (m *Memory) Ping(context.Context) error { return nil }
func (m *Memory) Close()                     {}

func (m *Memory) EnsureTenant(_ context.Context, id, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tenants[id]; !ok {
		m.tenants[id] = name
	}
	return nil
}

func (m *Memory) CreateUser(_ context.Context, u User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := strings.ToLower(u.Email)
	if _, ok := m.users[key]; ok {
		return ErrConflict
	}
	if _, ok := m.tenants[u.TenantID]; !ok {
		return ErrNotFound
	}
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}
	m.users[key] = u
	return nil
}

func (m *Memory) UserByEmail(_ context.Context, email string) (User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[strings.ToLower(email)]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (m *Memory) CountUsers(context.Context) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.users), nil
}

// userByIDLocked finds a user by id; callers hold m.mu.
func (m *Memory) userByIDLocked(id string) (string, User, bool) {
	for k, u := range m.users {
		if u.ID == id {
			return k, u, true
		}
	}
	return "", User{}, false
}

func (m *Memory) GetUser(_ context.Context, id string) (User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, u, ok := m.userByIDLocked(id); ok {
		return u, nil
	}
	return User{}, ErrNotFound
}

func (m *Memory) ListUsers(_ context.Context, tenantID string) ([]User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []User{}
	for _, u := range m.users {
		if tenantID == "" || u.TenantID == tenantID {
			out = append(out, u)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].Email < out[j].Email
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (m *Memory) UpdateUserPassword(_ context.Context, id, passwordHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k, u, ok := m.userByIDLocked(id)
	if !ok {
		return ErrNotFound
	}
	u.PasswordHash = passwordHash
	u.TokenVersion++
	m.users[k] = u
	return nil
}

func (m *Memory) SetUserActive(_ context.Context, id string, active bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k, u, ok := m.userByIDLocked(id)
	if !ok {
		return ErrNotFound
	}
	u.Active = active
	m.users[k] = u
	return nil
}

func (m *Memory) SetUserRoles(_ context.Context, id string, roles []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k, u, ok := m.userByIDLocked(id)
	if !ok {
		return ErrNotFound
	}
	u.Roles = append([]string(nil), roles...)
	u.TokenVersion++
	m.users[k] = u
	return nil
}

func (m *Memory) AddApproval(_ context.Context, a Approval) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.approvals[a.Token]; ok {
		return ErrConflict
	}
	m.approvals[a.Token] = a
	return nil
}

func (m *Memory) ListApprovals(_ context.Context, tenantID string) ([]Approval, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []Approval{}
	for _, a := range m.approvals {
		if tenantID == "" || a.TenantID == tenantID {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RequestedAt.After(out[j].RequestedAt) })
	return out, nil
}

func (m *Memory) GetApproval(_ context.Context, token string) (Approval, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.approvals[token]
	if !ok {
		return Approval{}, ErrNotFound
	}
	return a, nil
}

func (m *Memory) DecideApproval(_ context.Context, token, status, result, decidedBy string) (Approval, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.approvals[token]
	if !ok {
		return Approval{}, ErrNotFound
	}
	if a.Status != "PENDING" {
		return a, ErrConflict
	}
	now := time.Now()
	a.Status, a.ExecutionResult, a.DecidedBy, a.DecidedAt = status, result, decidedBy, &now
	m.approvals[token] = a
	return a, nil
}

func (m *Memory) PutSecret(_ context.Context, s Secret) (Secret, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := s.TenantID + "/" + s.KeyName
	now := time.Now()
	if old, ok := m.secrets[key]; ok {
		s.CreatedAt = old.CreatedAt
	} else {
		s.CreatedAt = now
	}
	s.UpdatedAt = now
	m.secrets[key] = s
	return s, nil
}

func (m *Memory) ListSecrets(_ context.Context, tenantID string) ([]Secret, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []Secret{}
	for _, s := range m.secrets {
		if s.TenantID == tenantID {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].KeyName < out[j].KeyName })
	return out, nil
}

func (m *Memory) GetSecret(_ context.Context, tenantID, keyName string) (Secret, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.secrets[tenantID+"/"+keyName]
	if !ok {
		return Secret{}, ErrNotFound
	}
	return s, nil
}

func (m *Memory) AppendAudit(_ context.Context, e AuditEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e.ID = int64(len(m.audit) + 1)
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	m.audit = append(m.audit, e)
	return nil
}

func (m *Memory) ListAudit(_ context.Context, tenantID string, limit int) ([]AuditEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []AuditEvent{}
	for i := len(m.audit) - 1; i >= 0 && (limit <= 0 || len(out) < limit); i-- {
		if tenantID == "" || m.audit[i].TenantID == tenantID {
			out = append(out, m.audit[i])
		}
	}
	return out, nil
}

func (m *Memory) GetRAGStrategy(_ context.Context, tenantID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.tenants[tenantID]; !ok {
		return "", ErrNotFound
	}
	if s, ok := m.ragStrat[tenantID]; ok {
		return s, nil
	}
	return string(tenant.DefaultRAGStrategy), nil
}

func (m *Memory) SetRAGStrategy(_ context.Context, tenantID, strategy string) error {
	if !tenant.ValidRAGStrategy(strategy) {
		return ErrInvalid
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tenants[tenantID]; !ok {
		return ErrNotFound
	}
	m.ragStrat[tenantID] = strategy
	return nil
}
