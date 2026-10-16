package application

import (
	"context"
	"strings"
	"time"

	"naosemeta.com/backend/internal/domain"
)

// AdminEnrollmentStore is used only by the trusted operator CLI, never by HTTP.
type AdminEnrollmentStore interface {
	Credentials(context.Context, string) (domain.Credential, error)
	// PromoteToAdmin compares the checked credentials, installs TOTP, consumes
	// its counter and revokes all previous sessions in one transaction.
	PromoteToAdmin(context.Context, string, string, []byte, int64, time.Time) error
}

func PromoteAdministrator(ctx context.Context, store AdminEnrollmentStore, passwords Passwords, second SecondFactor, clock Clock, username, password string, encrypted []byte, code string) error {
	username, err := domain.NormalizeUsername(username)
	if err != nil || domain.ValidatePassword(password) != nil {
		return domain.ErrUnauthenticated
	}
	credential, err := store.Credentials(ctx, username)
	if err != nil {
		return err
	}
	if credential.User.Role != "member" || len(credential.TOTPEncrypted) != 0 {
		return domain.ErrConflict
	}
	valid, err := passwords.Verify(ctx, password, credential.PasswordHash)
	if err != nil {
		return err
	}
	if !valid {
		return domain.ErrUnauthenticated
	}
	now := clock.Now()
	step, err := second.Verify(encrypted, strings.TrimSpace(code), now)
	if err != nil {
		return domain.ErrUnauthenticated
	}
	return store.PromoteToAdmin(ctx, credential.User.ID, credential.PasswordHash, encrypted, step, now)
}
