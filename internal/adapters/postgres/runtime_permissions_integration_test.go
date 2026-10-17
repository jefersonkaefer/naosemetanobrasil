//go:build integration

package postgres_test

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestDeploymentRuntimeRoleCanOperateWithoutSchemaOrHistoryAccess(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	body, err := os.ReadFile("../../../deploy/supabase-runtime.sql")
	if err != nil {
		t.Fatal(err)
	}
	// Keep the operator script's changes inside this test's rolled-back
	// transaction, including the role and all grants.
	sql := strings.Replace(string(body), "\nBEGIN;\n", "\n", 1)
	sql = strings.TrimSuffix(sql, "COMMIT;\n")
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, sql); err != nil {
		t.Fatal(err)
	}
	// Rerunning the deployment script must keep the same safe privileges.
	if _, err = tx.Exec(ctx, sql); err != nil {
		t.Fatal(err)
	}
	var privileged bool
	if err = tx.QueryRow(ctx, `SELECT rolcanlogin OR rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls FROM pg_roles WHERE rolname='nsm_api'`).Scan(&privileged); err != nil || privileged {
		t.Fatal("role gained privileges or login before private password setup", err)
	}
	if _, err = tx.Exec(ctx, `SET LOCAL ROLE nsm_api`); err != nil {
		t.Fatal(err)
	}
	var createSchema, history, readAudit, deleteUsers bool
	if err = tx.QueryRow(ctx, `SELECT has_schema_privilege(current_user,'app','CREATE'),has_table_privilege(current_user,'app.nsm_migrations','SELECT'),has_table_privilege(current_user,'app.audit_events','SELECT'),has_table_privilege(current_user,'app.users','DELETE')`).Scan(&createSchema, &history, &readAudit, &deleteUsers); err != nil || createSchema || history || readAudit || deleteUsers {
		t.Fatal("runtime role has excessive privileges", err)
	}
	if _, err = tx.Exec(ctx, `SELECT u.id,c.method,p.remote_user_id,r.message_id FROM app.users u LEFT JOIN app.sessions s ON s.user_id=u.id LEFT JOIN app.verification_challenges c ON c.user_id=u.id LEFT JOIN app.social_profiles p ON p.user_id=u.id LEFT JOIN app.instagram_message_receipts r ON r.challenge_id=c.id LIMIT 0`); err != nil {
		t.Fatal("readiness query denied", err)
	}
	id := f.secrets.NewID()
	name := "runtime_" + strings.ReplaceAll(id, "-", "")[:20]
	if _, err = tx.Exec(ctx, `INSERT INTO app.users(id,username,password_hash,role,created_at) VALUES($1,$2,'synthetic-test-only','member',now())`, id, name); err != nil {
		t.Fatal("registration insert denied", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO app.recovery_codes(user_id,code_hash) VALUES($1,$2)`, id, f.secrets.Digest(id)); err != nil {
		t.Fatal("recovery sequence denied", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO app.audit_events(actor_id,action,entity_id,created_at) VALUES($1,'account.created',$1,now())`, id); err != nil {
		t.Fatal("audit insert or sequence denied", err)
	}
}
