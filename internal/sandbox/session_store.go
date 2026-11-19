package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// sessionKeyPrefix namespaces sandbox sessions in Redis (SPEC-CORE-73.R11).
const sessionKeyPrefix = "aipods:sandbox:v1:"

// sessionOpTimeout bounds each Redis call.
const sessionOpTimeout = 500 * time.Millisecond

// SessionStore keeps sandbox sessions until their ExpiresAt.
type SessionStore interface {
	// Get returns the session, or ok=false when it does not exist or expired.
	Get(ctx context.Context, id string) (s SandboxSession, ok bool, err error)
	// Save writes the session; it expires at s.ExpiresAt.
	Save(ctx context.Context, s SandboxSession) error
}

// MemorySessionStore keeps sessions in process memory (development, tests).
type MemorySessionStore struct {
	mu       sync.Mutex
	sessions map[string]SandboxSession
	now      func() time.Time
}

func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{sessions: map[string]SandboxSession{}, now: time.Now}
}

func (m *MemorySessionStore) Get(_ context.Context, id string) (SandboxSession, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok || m.now().After(s.ExpiresAt) {
		delete(m.sessions, id)
		return SandboxSession{}, false, nil
	}
	return s, true, nil
}

func (m *MemorySessionStore) Save(_ context.Context, s SandboxSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.SessionID] = s
	return nil
}

// RedisSessionStore shares sessions between both colors (SPEC-CORE-73.R11).
// The client is owned by the caller.
type RedisSessionStore struct {
	client redis.UniversalClient
}

func NewRedisSessionStore(client redis.UniversalClient) *RedisSessionStore {
	return &RedisSessionStore{client: client}
}

func (r *RedisSessionStore) Get(ctx context.Context, id string) (SandboxSession, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, sessionOpTimeout)
	defer cancel()
	raw, err := r.client.Get(ctx, sessionKeyPrefix+id).Bytes()
	if errors.Is(err, redis.Nil) {
		return SandboxSession{}, false, nil
	}
	if err != nil {
		return SandboxSession{}, false, err
	}
	var s SandboxSession
	if err := json.Unmarshal(raw, &s); err != nil {
		return SandboxSession{}, false, err
	}
	return s, true, nil
}

func (r *RedisSessionStore) Save(ctx context.Context, s SandboxSession) error {
	ttl := time.Until(s.ExpiresAt)
	if ttl <= 0 {
		return nil
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, sessionOpTimeout)
	defer cancel()
	return r.client.Set(ctx, sessionKeyPrefix+s.SessionID, raw, ttl).Err()
}
