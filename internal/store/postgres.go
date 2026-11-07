package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migrationLockID serializes migrations when several core replicas start at once.
const migrationLockID = 6620670067

// Postgres is the persistent Store. All queries are parameterized (SPEC-CORE-67.R7).
type Postgres struct {
	pool *pgxpool.Pool
}

// OpenPostgres connects and applies pending migrations.
func OpenPostgres(ctx context.Context, url string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("store: connect: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	p := &Postgres{pool: pool}
	if err := p.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return p, nil
}

func (p *Postgres) migrate(ctx context.Context) error {
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationLockID); err != nil {
		return fmt.Errorf("store: migration lock: %w", err)
	}
	defer conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, migrationLockID)

	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("store: schema_migrations: %w", err)
	}

	names, err := fs.Glob(migrationsFS, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		version := strings.TrimSuffix(strings.TrimPrefix(name, "migrations/"), ".sql")
		var exists bool
		if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		sql, err := migrationsFS.ReadFile(name)
		if err != nil {
			return err
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("store: migration %s: %w", version, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (p *Postgres) Name() string                   { return "postgres" }
func (p *Postgres) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }
func (p *Postgres) Close()                         { p.pool.Close() }

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (p *Postgres) EnsureTenant(ctx context.Context, id, name string) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO tenants (id, name) VALUES ($1, $2) ON CONFLICT (id) DO NOTHING`, id, name)
	return err
}

func (p *Postgres) CreateUser(ctx context.Context, u User) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO users (id, tenant_id, email, password_hash, roles, active) VALUES ($1, $2, lower($3), $4, $5, $6)`,
		u.ID, u.TenantID, u.Email, u.PasswordHash, u.Roles, u.Active)
	switch {
	case isUniqueViolation(err):
		return ErrConflict
	case isForeignKeyViolation(err):
		return ErrNotFound
	}
	return err
}

func (p *Postgres) UserByEmail(ctx context.Context, email string) (User, error) {
	u, err := scanUser(p.pool.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE email = lower($1)`, email))
	return u, notFound(err)
}

func (p *Postgres) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := p.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, err
}

const userCols = `id, tenant_id, email, password_hash, roles, active, created_at, token_version`

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.TenantID, &u.Email, &u.PasswordHash, &u.Roles, &u.Active, &u.CreatedAt, &u.TokenVersion)
	return u, err
}

func (p *Postgres) GetUser(ctx context.Context, id string) (User, error) {
	u, err := scanUser(p.pool.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE id = $1`, id))
	return u, notFound(err)
}

func (p *Postgres) ListUsers(ctx context.Context, tenantID string) ([]User, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT `+userCols+` FROM users WHERE ($1 = '' OR tenant_id = $1) ORDER BY created_at, email`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (p *Postgres) UpdateUserPassword(ctx context.Context, id, passwordHash string) error {
	return execOne(p.pool.Exec(ctx, `UPDATE users SET password_hash = $2, token_version = token_version + 1 WHERE id = $1`, id, passwordHash))
}

func (p *Postgres) SetUserActive(ctx context.Context, id string, active bool) error {
	return execOne(p.pool.Exec(ctx, `UPDATE users SET active = $2 WHERE id = $1`, id, active))
}

// execOne maps "no row updated" to ErrNotFound.
func execOne(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

const approvalCols = `token, tenant_id, pod_id, action_name, summary, command, status, execution_result, decided_by, requested_at, decided_at`

func scanApproval(row pgx.Row) (Approval, error) {
	var a Approval
	err := row.Scan(&a.Token, &a.TenantID, &a.PodID, &a.ActionName, &a.Summary, &a.Command,
		&a.Status, &a.ExecutionResult, &a.DecidedBy, &a.RequestedAt, &a.DecidedAt)
	return a, err
}

func (p *Postgres) AddApproval(ctx context.Context, a Approval) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO approvals (token, tenant_id, pod_id, action_name, summary, command, status, requested_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		a.Token, a.TenantID, a.PodID, a.ActionName, a.Summary, a.Command, a.Status, a.RequestedAt)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

func (p *Postgres) ListApprovals(ctx context.Context, tenantID string) ([]Approval, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT `+approvalCols+` FROM approvals WHERE ($1 = '' OR tenant_id = $1) ORDER BY requested_at DESC LIMIT 500`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Approval{}
	for rows.Next() {
		a, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (p *Postgres) GetApproval(ctx context.Context, token string) (Approval, error) {
	a, err := scanApproval(p.pool.QueryRow(ctx, `SELECT `+approvalCols+` FROM approvals WHERE token = $1`, token))
	return a, notFound(err)
}

func (p *Postgres) DecideApproval(ctx context.Context, token, status, result, decidedBy string) (Approval, error) {
	a, err := scanApproval(p.pool.QueryRow(ctx,
		`UPDATE approvals SET status = $2, execution_result = $3, decided_by = $4, decided_at = now()
		 WHERE token = $1 AND status = 'PENDING' RETURNING `+approvalCols,
		token, status, result, decidedBy))
	if errors.Is(err, pgx.ErrNoRows) {
		existing, getErr := p.GetApproval(ctx, token)
		if getErr != nil {
			return Approval{}, getErr
		}
		return existing, ErrConflict
	}
	return a, err
}

func (p *Postgres) PutSecret(ctx context.Context, s Secret) (Secret, error) {
	err := p.pool.QueryRow(ctx,
		`INSERT INTO vault_secrets (tenant_id, key_name, cipher_text, masked_value, algorithm)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (tenant_id, key_name) DO UPDATE
		   SET cipher_text = EXCLUDED.cipher_text, masked_value = EXCLUDED.masked_value,
		       algorithm = EXCLUDED.algorithm, updated_at = now()
		 RETURNING created_at, updated_at`,
		s.TenantID, s.KeyName, s.CipherText, s.MaskedValue, s.Algorithm).Scan(&s.CreatedAt, &s.UpdatedAt)
	return s, err
}

func (p *Postgres) ListSecrets(ctx context.Context, tenantID string) ([]Secret, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT tenant_id, key_name, cipher_text, masked_value, algorithm, created_at, updated_at
		 FROM vault_secrets WHERE tenant_id = $1 ORDER BY key_name`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Secret{}
	for rows.Next() {
		var s Secret
		if err := rows.Scan(&s.TenantID, &s.KeyName, &s.CipherText, &s.MaskedValue, &s.Algorithm, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (p *Postgres) GetSecret(ctx context.Context, tenantID, keyName string) (Secret, error) {
	var s Secret
	err := p.pool.QueryRow(ctx,
		`SELECT tenant_id, key_name, cipher_text, masked_value, algorithm, created_at, updated_at
		 FROM vault_secrets WHERE tenant_id = $1 AND key_name = $2`, tenantID, keyName).
		Scan(&s.TenantID, &s.KeyName, &s.CipherText, &s.MaskedValue, &s.Algorithm, &s.CreatedAt, &s.UpdatedAt)
	return s, notFound(err)
}

func (p *Postgres) AppendAudit(ctx context.Context, e AuditEvent) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO audit_log (tenant_id, actor, action, target, outcome, client_ip, user_agent)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		e.TenantID, e.Actor, e.Action, e.Target, e.Outcome, e.ClientIP, e.UserAgent)
	return err
}

func (p *Postgres) ListAudit(ctx context.Context, tenantID string, limit int) ([]AuditEvent, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	rows, err := p.pool.Query(ctx,
		`SELECT id, tenant_id, actor, action, target, outcome, client_ip, user_agent, created_at
		 FROM audit_log WHERE ($1 = '' OR tenant_id = $1) ORDER BY id DESC LIMIT $2`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditEvent{}
	for rows.Next() {
		var e AuditEvent
		if err := rows.Scan(&e.ID, &e.TenantID, &e.Actor, &e.Action, &e.Target, &e.Outcome, &e.ClientIP, &e.UserAgent, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
