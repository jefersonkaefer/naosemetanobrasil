package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"naosemeta.com/backend/internal/domain"
)

func (s *Store) CreateAccount(ctx context.Context, c domain.Credential, recoveryHashes []string) error {
	return s.transaction(ctx, func(tx pgx.Tx) error {
		u := c.User
		if _, err := tx.Exec(ctx, `INSERT INTO app.users(id,username,display_name,password_hash,role,created_at,totp_encrypted) VALUES($1,$2,$3,$4,$5,$6,$7)`, u.ID, u.Username, u.DisplayName, c.PasswordHash, u.Role, u.CreatedAt, c.TOTPEncrypted); err != nil {
			return err
		}
		for _, hash := range recoveryHashes {
			if _, err := tx.Exec(ctx, `INSERT INTO app.recovery_codes(user_id,code_hash) VALUES($1,$2)`, u.ID, hash); err != nil {
				return err
			}
		}
		return audit(ctx, tx, u.ID, "account.created", u.ID, u.CreatedAt)
	})
}
func (s *Store) Credentials(ctx context.Context, username string) (domain.Credential, error) {
	var c domain.Credential
	err := s.pool.QueryRow(ctx, `SELECT id,username,display_name,role,created_at,password_hash,totp_encrypted FROM app.users WHERE username=$1 AND status='active'`, username).Scan(&c.User.ID, &c.User.Username, &c.User.DisplayName, &c.User.Role, &c.User.CreatedAt, &c.PasswordHash, &c.TOTPEncrypted)
	return c, translate(err)
}

// PromoteToAdmin is available only to the operator CLI. No public API calls it.
func (s *Store) PromoteToAdmin(ctx context.Context, id, expectedHash string, encrypted []byte, step int64, now time.Time) error {
	if len(encrypted) == 0 || step <= 0 {
		return domain.ErrInvalid
	}
	return s.transaction(ctx, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `UPDATE app.users SET role='admin',totp_encrypted=$3,totp_last_step=$4 WHERE id=$1 AND password_hash=$2 AND role='member' AND status='active' AND totp_encrypted IS NULL`, id, expectedHash, encrypted, step)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return domain.ErrConflict
		}
		if _, err = tx.Exec(ctx, `UPDATE app.sessions SET revoked_at=$2 WHERE user_id=$1 AND revoked_at IS NULL`, id, now); err != nil {
			return err
		}
		return audit(ctx, tx, id, "account.admin_enrolled", id, now)
	})
}
func (s *Store) Recover(ctx context.Context, username, codeHash, passwordHash string, now time.Time) error {
	return s.transaction(ctx, func(tx pgx.Tx) error {
		var id string
		if err := tx.QueryRow(ctx, `SELECT id FROM app.users WHERE username=$1 AND status='active' FOR UPDATE`, username).Scan(&id); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrUnauthenticated
			}
			return err
		}
		result, err := tx.Exec(ctx, `UPDATE app.recovery_codes SET used_at=$3 WHERE user_id=$1 AND code_hash=$2 AND used_at IS NULL`, id, codeHash, now)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return domain.ErrUnauthenticated
		}
		if _, err = tx.Exec(ctx, `UPDATE app.users SET password_hash=$2 WHERE id=$1`, id, passwordHash); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE app.sessions SET revoked_at=$2 WHERE user_id=$1 AND revoked_at IS NULL`, id, now); err != nil {
			return err
		}
		return audit(ctx, tx, id, "account.recovered", id, now)
	})
}
func (s *Store) CreateSession(ctx context.Context, session domain.Session, expectedPasswordHash string, step *int64) error {
	return s.transaction(ctx, func(tx pgx.Tx) error {
		var role, status, hash string
		var lastStep int64
		if err := tx.QueryRow(ctx, `SELECT role,status,password_hash,totp_last_step FROM app.users WHERE id=$1 FOR UPDATE`, session.UserID).Scan(&role, &status, &hash, &lastStep); err != nil {
			return err
		}
		if status != "active" || hash != expectedPasswordHash {
			return domain.ErrUnauthenticated
		}
		if role == "admin" {
			if step == nil || *step <= lastStep {
				return domain.ErrUnauthenticated
			}
			if _, err := tx.Exec(ctx, `UPDATE app.users SET totp_last_step=$2 WHERE id=$1`, session.UserID, *step); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `INSERT INTO app.sessions(id,user_id,token_hash,expires_at) VALUES($1,$2,$3,$4)`, session.ID, session.UserID, session.TokenHash, session.ExpiresAt)
		return err
	})
}
func (s *Store) SessionUser(ctx context.Context, hash string, now time.Time) (domain.User, error) {
	var u domain.User
	err := s.pool.QueryRow(ctx, `SELECT u.id,u.username,u.display_name,u.role,u.created_at FROM app.sessions s JOIN app.users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>$2 AND u.status='active'`, hash, now).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, domain.ErrUnauthenticated
	}
	return u, err
}
func (s *Store) RevokeSession(ctx context.Context, hash string) error {
	_, err := s.pool.Exec(ctx, `UPDATE app.sessions SET revoked_at=now() WHERE token_hash=$1 AND revoked_at IS NULL`, hash)
	return err
}
