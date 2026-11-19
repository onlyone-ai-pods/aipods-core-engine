package store

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

// Sagas and dynamic pods (SPEC-CORE-73.R9, R10).

func (m *Memory) CreateSaga(_ context.Context, s Saga) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sagas[s.SagaID]; ok {
		return ErrConflict
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	m.sagas[s.SagaID] = s
	return nil
}

func (m *Memory) GetSaga(_ context.Context, sagaID string) (Saga, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sagas[sagaID]
	if !ok {
		return Saga{}, ErrNotFound
	}
	return s, nil
}

func (m *Memory) FinishSaga(_ context.Context, sagaID, status string) (Saga, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sagas[sagaID]
	if !ok {
		return Saga{}, ErrNotFound
	}
	if s.Status != SagaAwaitingOTP {
		return s, ErrConflict
	}
	now := time.Now()
	s.Status = status
	if status == "COMPLETED" {
		s.ExecutedAt = &now
	} else {
		s.CompensatedAt = &now
	}
	m.sagas[sagaID] = s
	return s, nil
}

func (m *Memory) UpsertDynamicPod(_ context.Context, p DynamicPod) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p.Keywords = append([]string(nil), p.Keywords...)
	p.UpdatedAt = time.Now()
	m.pods[p.PodID] = p
	return nil
}

func (m *Memory) ListDynamicPods(context.Context) ([]DynamicPod, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]DynamicPod, 0, len(m.pods))
	for _, p := range m.pods {
		p.Keywords = append([]string(nil), p.Keywords...)
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PodID < out[j].PodID })
	return out, nil
}

const sagaCols = `saga_id, tenant_id, action_name, is_critical, status, created_at, executed_at, compensated_at`

func scanSaga(row pgx.Row) (Saga, error) {
	var s Saga
	err := row.Scan(&s.SagaID, &s.TenantID, &s.ActionName, &s.IsCritical, &s.Status,
		&s.CreatedAt, &s.ExecutedAt, &s.CompensatedAt)
	return s, err
}

func (p *Postgres) CreateSaga(ctx context.Context, s Saga) error {
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	_, err := p.pool.Exec(ctx,
		`INSERT INTO sagas (`+sagaCols+`) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		s.SagaID, s.TenantID, s.ActionName, s.IsCritical, s.Status, s.CreatedAt, s.ExecutedAt, s.CompensatedAt)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

func (p *Postgres) GetSaga(ctx context.Context, sagaID string) (Saga, error) {
	s, err := scanSaga(p.pool.QueryRow(ctx, `SELECT `+sagaCols+` FROM sagas WHERE saga_id = $1`, sagaID))
	return s, notFound(err)
}

func (p *Postgres) FinishSaga(ctx context.Context, sagaID, status string) (Saga, error) {
	s, err := scanSaga(p.pool.QueryRow(ctx,
		`UPDATE sagas SET status = $2,
		        executed_at    = CASE WHEN $2 = 'COMPLETED' THEN now() ELSE executed_at END,
		        compensated_at = CASE WHEN $2 = 'COMPLETED' THEN compensated_at ELSE now() END
		 WHERE saga_id = $1 AND status = 'AWAITING_2FA_OTP' RETURNING `+sagaCols,
		sagaID, status))
	if errors.Is(err, pgx.ErrNoRows) {
		existing, getErr := p.GetSaga(ctx, sagaID)
		if getErr != nil {
			return Saga{}, getErr
		}
		return existing, ErrConflict
	}
	return s, err
}

func (p *Postgres) UpsertDynamicPod(ctx context.Context, d DynamicPod) error {
	keywords := d.Keywords
	if keywords == nil {
		keywords = []string{}
	}
	_, err := p.pool.Exec(ctx,
		`INSERT INTO dynamic_pods (pod_id, tenant_id, name, endpoint_url, keywords, status)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (pod_id) DO UPDATE
		   SET tenant_id = EXCLUDED.tenant_id, name = EXCLUDED.name, endpoint_url = EXCLUDED.endpoint_url,
		       keywords = EXCLUDED.keywords, status = EXCLUDED.status, updated_at = now()`,
		d.PodID, d.TenantID, d.Name, d.EndpointURL, keywords, d.Status)
	return err
}

func (p *Postgres) ListDynamicPods(ctx context.Context) ([]DynamicPod, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT pod_id, tenant_id, name, endpoint_url, keywords, status, updated_at FROM dynamic_pods ORDER BY pod_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DynamicPod{}
	for rows.Next() {
		var d DynamicPod
		if err := rows.Scan(&d.PodID, &d.TenantID, &d.Name, &d.EndpointURL, &d.Keywords, &d.Status, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
