package store

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// TEST_DATABASE_URL enables the Postgres run, e.g.
// postgres://aipods@127.0.0.1:55432/aipods_test?sslmode=disable
func stores(t *testing.T) map[string]func() Store {
	out := map[string]func() Store{"memory": func() Store { return NewMemory() }}
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		out["postgres"] = func() Store {
			p, err := OpenPostgres(context.Background(), url)
			if err != nil {
				t.Fatalf("postgres: %v", err)
			}
			for _, tbl := range []string{"approvals", "vault_secrets", "users"} {
				if _, err := p.pool.Exec(context.Background(), "DELETE FROM "+tbl); err != nil {
					t.Fatal(err)
				}
			}
			// audit_log is append-only: truncate bypasses the row trigger, only in tests.
			if _, err := p.pool.Exec(context.Background(), "TRUNCATE audit_log"); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(p.Close)
			return p
		}
	} else {
		t.Log("TEST_DATABASE_URL not set: skipping Postgres store tests")
	}
	return out
}

func TestStoreContract(t *testing.T) {
	ctx := context.Background()
	for name, open := range stores(t) {
		t.Run(name, func(t *testing.T) {
			s := open()

			// Tenants and users
			if err := s.EnsureTenant(ctx, "tenant_acme", "Acme SA"); err != nil {
				t.Fatal(err)
			}
			if err := s.EnsureTenant(ctx, "tenant_acme", "otro nombre"); err != nil {
				t.Fatal("EnsureTenant must be idempotent:", err)
			}
			u := User{ID: "u1", TenantID: "tenant_acme", Email: "Ana@Acme.com", PasswordHash: "h", Roles: []string{"customer"}, Active: true}
			if err := s.CreateUser(ctx, u); err != nil {
				t.Fatal(err)
			}
			if err := s.CreateUser(ctx, User{ID: "u2", TenantID: "tenant_acme", Email: "ana@acme.com", PasswordHash: "h", Roles: []string{"customer"}}); !errors.Is(err, ErrConflict) {
				t.Errorf("duplicate email (case-insensitive) should conflict, got %v", err)
			}
			if err := s.CreateUser(ctx, User{ID: "u3", TenantID: "no_existe", Email: "x@y.com", PasswordHash: "h", Roles: []string{"customer"}}); !errors.Is(err, ErrNotFound) {
				t.Errorf("unknown tenant should be ErrNotFound, got %v", err)
			}
			got, err := s.UserByEmail(ctx, "ANA@acme.com")
			if err != nil || got.ID != "u1" || len(got.Roles) != 1 {
				t.Fatalf("UserByEmail: %+v %v", got, err)
			}
			if n, _ := s.CountUsers(ctx); n != 1 {
				t.Errorf("expected 1 user, got %d", n)
			}

			// Approvals
			now := time.Now().UTC().Truncate(time.Millisecond)
			for i, tn := range []string{"tenant_acme", "tenant_otro"} {
				err := s.AddApproval(ctx, Approval{Token: "tok" + tn, TenantID: tn, PodID: "POD_X", ActionName: "a",
					Summary: "s", Command: "c", Status: "PENDING", RequestedAt: now.Add(time.Duration(i) * time.Second)})
				if err != nil {
					t.Fatal(err)
				}
			}
			if list, _ := s.ListApprovals(ctx, "tenant_acme"); len(list) != 1 || list[0].TenantID != "tenant_acme" {
				t.Errorf("tenant filter broken: %+v", list)
			}
			if list, _ := s.ListApprovals(ctx, ""); len(list) != 2 || list[0].TenantID != "tenant_otro" {
				t.Errorf("expected 2 approvals, newest first: %+v", list)
			}
			a, err := s.DecideApproval(ctx, "toktenant_acme", "APPROVED", "ok", "u1")
			if err != nil || a.Status != "APPROVED" || a.DecidedBy != "u1" || a.DecidedAt == nil {
				t.Fatalf("DecideApproval: %+v %v", a, err)
			}
			if _, err := s.DecideApproval(ctx, "toktenant_acme", "REJECTED", "", "u1"); !errors.Is(err, ErrConflict) {
				t.Errorf("deciding twice should conflict, got %v", err)
			}
			if _, err := s.DecideApproval(ctx, "nope", "REJECTED", "", "u1"); !errors.Is(err, ErrNotFound) {
				t.Errorf("unknown token should be ErrNotFound, got %v", err)
			}

			// Secrets are scoped by tenant
			if _, err := s.PutSecret(ctx, Secret{TenantID: "tenant_acme", KeyName: "K", CipherText: "c1", MaskedValue: "m", Algorithm: "AES-256-GCM"}); err != nil {
				t.Fatal(err)
			}
			if _, err := s.PutSecret(ctx, Secret{TenantID: "tenant_acme", KeyName: "K", CipherText: "c2", MaskedValue: "m", Algorithm: "AES-256-GCM"}); err != nil {
				t.Fatal(err)
			}
			if sec, err := s.GetSecret(ctx, "tenant_acme", "K"); err != nil || sec.CipherText != "c2" {
				t.Errorf("upsert failed: %+v %v", sec, err)
			}
			if _, err := s.GetSecret(ctx, "tenant_otro", "K"); !errors.Is(err, ErrNotFound) {
				t.Errorf("SECURITY: other tenant read a secret, err=%v", err)
			}
			if list, _ := s.ListSecrets(ctx, "tenant_acme"); len(list) != 1 {
				t.Errorf("expected 1 secret, got %d", len(list))
			}

			// Audit
			for _, act := range []string{"login", "vault.reveal"} {
				if err := s.AppendAudit(ctx, AuditEvent{TenantID: "tenant_acme", Actor: "u1", Action: act, Outcome: "success"}); err != nil {
					t.Fatal(err)
				}
			}
			if ev, _ := s.ListAudit(ctx, "tenant_acme", 10); len(ev) != 2 || ev[0].Action != "vault.reveal" {
				t.Errorf("audit newest first: %+v", ev)
			}
		})
	}
}

func TestPostgresPersistsAcrossInstancesAndAuditIsAppendOnly(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	p1, err := OpenPostgres(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	tok := "persist_" + time.Now().Format("150405.000000")
	if err := p1.AddApproval(ctx, Approval{Token: tok, TenantID: "t", PodID: "P", ActionName: "a", Summary: "s", Command: "c", Status: "PENDING", RequestedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	_ = p1.AppendAudit(ctx, AuditEvent{TenantID: "t", Actor: "x", Action: "probe", Outcome: "success"})
	if _, err := p1.pool.Exec(ctx, `UPDATE audit_log SET actor = 'hacker'`); err == nil {
		t.Error("audit_log must reject UPDATE")
	}
	p1.Close()

	// A second instance (= core restart) re-runs migrations idempotently and sees the data.
	p2, err := OpenPostgres(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer p2.Close()
	if _, err := p2.GetApproval(ctx, tok); err != nil {
		t.Errorf("approval lost across instances: %v", err)
	}
}
