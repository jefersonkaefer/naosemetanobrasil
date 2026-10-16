package migrations

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed *.sql
var files embed.FS

// Up applies embedded migrations atomically and rejects edited migration history.
// Run with a migration role, never from the public API startup.
func Up(ctx context.Context, pool *pgxpool.Pool) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(61411620261016)`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
CREATE SCHEMA IF NOT EXISTS app;
DO $$ BEGIN
    IF to_regclass('public.nsm_migrations') IS NOT NULL THEN
        IF to_regclass('app.nsm_migrations') IS NOT NULL THEN
            RAISE EXCEPTION 'ambiguous migration history';
        END IF;
        ALTER TABLE public.nsm_migrations SET SCHEMA app;
    END IF;
END $$;
CREATE TABLE IF NOT EXISTS app.nsm_migrations (name text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now());
ALTER TABLE app.nsm_migrations ENABLE ROW LEVEL SECURITY;
REVOKE ALL ON app.nsm_migrations FROM PUBLIC;
DO $$ DECLARE role_name text; BEGIN
    FOREACH role_name IN ARRAY ARRAY['anon','authenticated','service_role'] LOOP
        IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname=role_name) AND role_name<>current_user THEN
            EXECUTE format('REVOKE ALL ON app.nsm_migrations FROM %I',role_name);
        END IF;
    END LOOP;
END $$;`); err != nil {
			return err
		}
		entries, err := fs.Glob(files, "*.sql")
		if err != nil {
			return err
		}
		for _, name := range entries {
			body, err := files.ReadFile(name)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(body)
			checksum := hex.EncodeToString(sum[:])
			var old string
			err = tx.QueryRow(ctx, `SELECT checksum FROM app.nsm_migrations WHERE name=$1`, name).Scan(&old)
			if err == nil {
				if old != checksum {
					return fmt.Errorf("migration checksum mismatch: %s", name)
				}
				continue
			}
			if err != pgx.ErrNoRows {
				return err
			}
			if _, err = tx.Exec(ctx, string(body)); err != nil {
				return fmt.Errorf("migration %s: %w", name, err)
			}
			if _, err = tx.Exec(ctx, `INSERT INTO app.nsm_migrations(name,checksum) VALUES($1,$2)`, name, checksum); err != nil {
				return err
			}
		}
		return nil
	})
}
