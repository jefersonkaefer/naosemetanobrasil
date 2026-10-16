//go:build integration

package postgres_test

import (
	"context"
	"github.com/jackc/pgx/v5"
	"naosemeta.com/backend/migrations"
	"strings"
	"testing"
)

func TestMigrationHistoryMovesFromPublicAndDeniesUnprivilegedAccess(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	// Emulate the previous release; the next Up must relocate the exact
	// existing history, retaining all checksums rather than replaying scripts.
	if _, err := f.pool.Exec(ctx, `ALTER TABLE app.nsm_migrations SET SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Up(ctx, f.pool); err != nil {
		t.Fatal(err)
	}
	var private, public bool
	if err := f.pool.QueryRow(ctx, `SELECT to_regclass('app.nsm_migrations') IS NOT NULL,to_regclass('public.nsm_migrations') IS NOT NULL`).Scan(&private, &public); err != nil || !private || public {
		t.Fatal("history not private", err)
	}
	role := "audit_" + strings.ReplaceAll(f.secrets.NewID(), "-", "")
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	identifier := pgx.Identifier{role}.Sanitize()
	for _, sql := range []string{"CREATE ROLE " + identifier, "GRANT USAGE ON SCHEMA app TO " + identifier, "GRANT SELECT ON app.nsm_migrations TO " + identifier, "SET LOCAL ROLE " + identifier} {
		if _, err = tx.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	// Even a future broad SELECT grant must not expose the history: RLS has
	// no non-owner policy on this table.
	var visible int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM app.nsm_migrations`).Scan(&visible); err != nil || visible != 0 {
		t.Fatal("unprivileged role can read history", err, visible)
	}
}
