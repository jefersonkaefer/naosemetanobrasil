//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"naosemeta.com/backend/internal/application"
	"naosemeta.com/backend/internal/domain"
)

func TestAdminEnrollmentPreservesAccountAndRequiresFreshLogin(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	r := f.member(t)
	before, err := f.store.Credentials(ctx, r.User.Username)
	if err != nil {
		t.Fatal(err)
	}
	login, err := f.auth.Login(ctx, r.User.Username, "integration-password-long", "")
	if err != nil {
		t.Fatal(err)
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "Test", AccountName: r.User.Username})
	if err != nil {
		t.Fatal(err)
	}
	code, err := totp.GenerateCode(key.Secret(), f.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	encrypted := f.second.Seal(key.Secret())
	promote := func(password, otp string) error {
		return application.PromoteAdministrator(ctx, f.store, f.passwords, f.second, f.clock, r.User.Username, password, encrypted, otp)
	}
	for _, input := range [][2]string{{"incorrect-password-long", code}, {"integration-password-long", "bad"}} {
		if err := promote(input[0], input[1]); !errors.Is(err, domain.ErrUnauthenticated) {
			t.Fatalf("accepted invalid enrollment: %v", err)
		}
		if u, err := f.auth.Authenticate(ctx, login.Token); err != nil || u.Role != "member" {
			t.Fatal("failed enrollment changed access", err)
		}
	}
	if err := f.store.PromoteToAdmin(ctx, r.User.ID, "stale-password-hash", encrypted, f.clock.Now().Unix()/30, f.clock.Now()); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("accepted changed credentials", err)
	}
	if err := promote("integration-password-long", code); err != nil {
		t.Fatal(err)
	}
	after, err := f.store.Credentials(ctx, r.User.Username)
	if err != nil || after.User.ID != before.User.ID || after.PasswordHash != before.PasswordHash || after.User.Role != "admin" {
		t.Fatal("account identity or password not preserved", err)
	}
	var recoveryCount, auditCount int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM app.recovery_codes WHERE user_id=$1 AND used_at IS NULL`, r.User.ID).Scan(&recoveryCount); err != nil || recoveryCount != len(r.RecoveryCodes) {
		t.Fatal("recovery codes not preserved", err)
	}
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM app.audit_events WHERE actor_id=$1 AND action='account.admin_enrolled'`, r.User.ID).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatal("missing enrollment audit", err)
	}
	if _, err := f.auth.Authenticate(ctx, login.Token); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatal("old member session gained admin privileges", err)
	}
	for _, otp := range []string{"", code} {
		if _, err := f.auth.Login(ctx, r.User.Username, "integration-password-long", otp); !errors.Is(err, domain.ErrUnauthenticated) {
			t.Fatal("accepted missing or reused TOTP", err)
		}
	}
	f.clock.now = f.clock.now.Add(30 * time.Second)
	next, err := totp.GenerateCode(key.Secret(), f.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if result, err := f.auth.Login(ctx, r.User.Username, "integration-password-long", next); err != nil || result.User.Role != "admin" {
		t.Fatal("fresh administrator login failed", err)
	}
	if err := promote("integration-password-long", next); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("allowed overwriting enrolled administrator", err)
	}
}
