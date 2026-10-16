package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"naosemeta.com/backend/internal/application"
	"naosemeta.com/backend/internal/domain"
)

type Store struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Store { return &Store{pool} }

// Ready checks both connectivity and the identity schema without reading user data.
func (s *Store) Ready(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `SELECT u.id,c.method,p.remote_user_id,r.message_id FROM app.users u LEFT JOIN app.sessions s ON s.user_id=u.id LEFT JOIN app.verification_challenges c ON c.user_id=u.id LEFT JOIN app.social_profiles p ON p.user_id=u.id LEFT JOIN app.instagram_message_receipts r ON r.challenge_id=c.id LIMIT 0`)
	return err
}

var (
	_ application.AccountStore          = (*Store)(nil)
	_ application.SessionStore          = (*Store)(nil)
	_ application.VerificationStore     = (*Store)(nil)
	_ application.InstagramMessageStore = (*Store)(nil)
)

func translate(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) && pgerr.Code == "23505" {
		return domain.ErrConflict
	}
	return err
}
func (s *Store) transaction(ctx context.Context, fn func(pgx.Tx) error) error {
	return translate(pgx.BeginFunc(ctx, s.pool, fn))
}
func lockUser(ctx context.Context, tx pgx.Tx, id string) error {
	var active bool
	if err := tx.QueryRow(ctx, `SELECT status='active' FROM app.users WHERE id=$1 FOR UPDATE`, id).Scan(&active); err != nil {
		return translate(err)
	}
	if !active {
		return domain.ErrForbidden
	}
	return nil
}
func audit(ctx context.Context, tx pgx.Tx, actor, action, entity string, now time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO app.audit_events(actor_id,action,entity_id,created_at) VALUES($1,$2,$3,$4)`, actor, action, entity, now)
	return err
}
