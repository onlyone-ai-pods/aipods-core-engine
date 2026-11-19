package store

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestSPEC_CORE_73_R14_StartupMigrationsAreExpandOnly(t *testing.T) {
	names, err := fs.Glob(migrationsFS, "migrations/*.sql")
	if err != nil || len(names) == 0 {
		t.Fatalf("no startup migrations found: %v", err)
	}
	for _, name := range names {
		sql, err := migrationsFS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if v := ExpandOnlyViolations(string(sql)); len(v) > 0 {
			t.Errorf("%s runs at startup but contains %v: move it to migrations/contract/ (SPEC-CORE-73.R14, R16)", name, v)
		}
	}
}

func TestSPEC_CORE_73_R14_PolicyRejectsContractOperations(t *testing.T) {
	for sql, want := range map[string]string{
		"ALTER TABLE users DROP COLUMN legacy;":              "DROP TABLE/COLUMN/INDEX",
		"drop table approvals;":                              "DROP TABLE/COLUMN/INDEX",
		"DROP INDEX idx_x;":                                  "DROP TABLE/COLUMN/INDEX",
		"ALTER TABLE users RENAME COLUMN email TO mail;":     "RENAME",
		"ALTER TABLE users RENAME TO people;":                "RENAME",
		"ALTER TABLE users ALTER COLUMN roles TYPE jsonb;":   "ALTER COLUMN ... TYPE",
		"ALTER TABLE users ALTER COLUMN email SET NOT NULL;": "ALTER COLUMN ... SET NOT NULL",
		"INSERT INTO t2 SELECT * FROM t1;":                   "SELECT *",
	} {
		if v := ExpandOnlyViolations(sql); len(v) != 1 || v[0] != want {
			t.Errorf("%q: violations %v, want [%s]", sql, v, want)
		}
	}
	for _, ok := range []string{
		"ALTER TABLE users ADD COLUMN token_version INTEGER NOT NULL DEFAULT 0;",
		"CREATE TABLE IF NOT EXISTS sagas (saga_id UUID PRIMARY KEY);",
		"CREATE INDEX IF NOT EXISTS idx_sagas_tenant ON sagas (tenant_id);",
		"-- DROP COLUMN in a comment is fine\nSELECT 1;",
	} {
		if v := ExpandOnlyViolations(ok); len(v) > 0 {
			t.Errorf("%q is expand-only but got %v", ok, v)
		}
	}
}

func openTestPostgres(t *testing.T) *Postgres {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set: skipping Postgres migration tests")
	}
	p, err := OpenPostgres(context.Background(), url)
	if err != nil {
		t.Fatalf("postgres: %v", err)
	}
	t.Cleanup(p.Close)
	return p
}

// A table locked by the color still serving makes the new migration fail in
// about lock_timeout, naming the migration, instead of queueing behind it.
func TestSPEC_CORE_73_R15_MigrationFailsFastOnLock(t *testing.T) {
	p := openTestPostgres(t)
	ctx := context.Background()
	tag := fmt.Sprintf("r15_%d", time.Now().UnixNano())
	table := "lock_probe_" + tag
	if _, err := p.pool.Exec(ctx, "CREATE TABLE "+table+" (id INT)"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = p.pool.Exec(context.Background(), "DROP TABLE IF EXISTS "+table)
		_, _ = p.pool.Exec(context.Background(), "DELETE FROM schema_migrations WHERE version LIKE $1", "%"+tag+"%")
	})

	holder, err := p.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(context.Background()) }()
	if _, err := holder.Exec(ctx, "LOCK TABLE "+table+" IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}

	version := "9001_" + tag
	fsys := fstest.MapFS{"m/" + version + ".sql": {Data: []byte("ALTER TABLE " + table + " ADD COLUMN note TEXT;")}}
	start := time.Now()
	_, err = p.applyMigrations(ctx, fsys, "m", "")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("migration should fail while the table is locked")
	}
	if !strings.Contains(err.Error(), version) || !strings.Contains(err.Error(), "lock") {
		t.Errorf("error should name the migration and the lock: %v", err)
	}
	if elapsed < 4*time.Second || elapsed > 15*time.Second {
		t.Errorf("failed after %s, want about 5s", elapsed)
	}
	var recorded bool
	_ = p.pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", version).Scan(&recorded)
	if recorded {
		t.Error("a failed migration must not be recorded")
	}
}

func TestSPEC_CORE_73_R16_ContractMigrationsRunOnlyOnDemand(t *testing.T) {
	p := openTestPostgres(t)
	ctx := context.Background()
	tag := fmt.Sprintf("r16_%d", time.Now().UnixNano())
	table := "contract_probe_" + tag
	t.Cleanup(func() {
		_, _ = p.pool.Exec(context.Background(), "DROP TABLE IF EXISTS "+table)
		_, _ = p.pool.Exec(context.Background(), "DELETE FROM schema_migrations WHERE version LIKE $1", "%"+tag+"%")
	})
	fsys := fstest.MapFS{
		"m/0001_" + tag + ".sql":                {Data: []byte("CREATE TABLE " + table + " (id INT, legacy TEXT);")},
		"m/contract/0001_" + tag + "_drop.sql":  {Data: []byte("ALTER TABLE " + table + " DROP COLUMN legacy;")},
		"m/contract/0002_" + tag + "_index.sql": {Data: []byte("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_" + tag + " ON " + table + " (id);")},
	}

	// Startup: only the expand file.
	applied, err := p.applyMigrations(ctx, fsys, "m", "")
	if err != nil || len(applied) != 1 {
		t.Fatalf("startup applied %v, err %v; want only the expand file", applied, err)
	}
	if !columnExists(t, p, table, "legacy") {
		t.Fatal("startup must not run contract migrations")
	}

	// aipods-cli migrate contract: both files, once, with the contract/ prefix.
	applied, err = p.applyMigrations(ctx, fsys, "m/contract", contractPrefix)
	if err != nil || len(applied) != 2 || !strings.HasPrefix(applied[0], "contract/") {
		t.Fatalf("contract applied %v, err %v", applied, err)
	}
	if columnExists(t, p, table, "legacy") {
		t.Fatal("contract migration did not drop the column")
	}
	applied, err = p.applyMigrations(ctx, fsys, "m/contract", contractPrefix)
	if err != nil || len(applied) != 0 {
		t.Fatalf("second run applied %v, err %v; want none", applied, err)
	}
}

func columnExists(t *testing.T, p *Postgres, table, column string) bool {
	t.Helper()
	var ok bool
	if err := p.pool.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = $1 AND column_name = $2)`,
		table, column).Scan(&ok); err != nil {
		t.Fatal(err)
	}
	return ok
}

func TestSPEC_CORE_73_R10_DynamicPodsPersist(t *testing.T) {
	for name, open := range stores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			s := open()
			id := fmt.Sprintf("POD_R10_%d", time.Now().UnixNano())
			if err := s.UpsertDynamicPod(ctx, DynamicPod{PodID: id, TenantID: "t1", Name: "a", EndpointURL: "http://a", Keywords: []string{"k1"}}); err != nil {
				t.Fatal(err)
			}
			if err := s.UpsertDynamicPod(ctx, DynamicPod{PodID: id, TenantID: "t1", Name: "b", EndpointURL: "http://b"}); err != nil {
				t.Fatal(err)
			}
			pods, err := s.ListDynamicPods(ctx)
			if err != nil {
				t.Fatal(err)
			}
			found := 0
			for _, p := range pods {
				if p.PodID == id {
					found++
					if p.Name != "b" || p.EndpointURL != "http://b" || len(p.Keywords) != 0 {
						t.Errorf("pod = %+v, want the second version", p)
					}
				}
			}
			if found != 1 {
				t.Fatalf("found %d rows for %s", found, id)
			}
		})
	}
}
