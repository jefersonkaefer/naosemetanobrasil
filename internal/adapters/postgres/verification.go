package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"naosemeta.com/backend/internal/domain"
)

const challengeColumns = `id,user_id,expected_handle,code_hash,status,created_at,expires_at,sent_at,observed_handle,COALESCE(reviewed_by::text,''),review_reference,method,observed_account_id,observed_sender_id`

func scanChallenge(row pgx.Row) (domain.Challenge, error) {
	var c domain.Challenge
	err := row.Scan(&c.ID, &c.UserID, &c.ExpectedHandle, &c.CodeHash, &c.Status, &c.CreatedAt, &c.ExpiresAt, &c.SentAt, &c.ObservedHandle, &c.ReviewedBy, &c.ReviewReference, &c.Method, &c.ObservedAccountID, &c.ObservedSenderID)
	return c, translate(err)
}
func (s *Store) IssueChallenge(ctx context.Context, c domain.Challenge) error {
	return s.transaction(ctx, func(tx pgx.Tx) error {
		if err := lockUser(ctx, tx, c.UserID); err != nil {
			return err
		}
		var linked bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.social_profiles WHERE user_id=$1 AND revoked_at IS NULL)`, c.UserID).Scan(&linked); err != nil {
			return err
		}
		if linked {
			return domain.ErrConflict
		}
		if _, err := tx.Exec(ctx, `UPDATE app.verification_challenges SET status='cancelled' WHERE user_id=$1 AND status IN ('pending','awaiting_confirmation')`, c.UserID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO app.verification_challenges(id,user_id,expected_handle,code_hash,status,created_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, c.ID, c.UserID, c.ExpectedHandle, c.CodeHash, c.Status, c.CreatedAt, c.ExpiresAt); err != nil {
			return err
		}
		return audit(ctx, tx, c.UserID, "instagram.challenge_issued", c.ID, c.CreatedAt)
	})
}
func (s *Store) Challenge(ctx context.Context, userID, id string) (domain.Challenge, error) {
	return scanChallenge(s.pool.QueryRow(ctx, `SELECT `+challengeColumns+` FROM app.verification_challenges WHERE id=$1 AND user_id=$2`, id, userID))
}
func (s *Store) MarkSent(ctx context.Context, userID, id string, now time.Time) error {
	return s.transaction(ctx, func(tx pgx.Tx) error {
		c, err := scanChallenge(tx.QueryRow(ctx, `SELECT `+challengeColumns+` FROM app.verification_challenges WHERE id=$1 AND user_id=$2 FOR UPDATE`, id, userID))
		if err != nil {
			return err
		}
		if err = c.MarkSent(now); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE app.verification_challenges SET sent_at=$2 WHERE id=$1`, c.ID, c.SentAt)
		return err
	})
}
func (s *Store) Observe(ctx context.Context, actor, hash, handle, reference string, now time.Time) (domain.Challenge, error) {
	var c domain.Challenge
	err := s.transaction(ctx, func(tx pgx.Tx) error {
		var err error
		c, err = scanChallenge(tx.QueryRow(ctx, `SELECT `+challengeColumns+` FROM app.verification_challenges WHERE code_hash=$1 FOR UPDATE`, hash))
		if err != nil {
			return err
		}
		if err = c.Observe(handle, actor, reference, now); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE app.verification_challenges SET status=$2,observed_handle=$3,reviewed_by=$4,review_reference=$5 WHERE id=$1`, c.ID, c.Status, c.ObservedHandle, c.ReviewedBy, c.ReviewReference); err != nil {
			return err
		}
		return audit(ctx, tx, actor, "instagram.message_observed", c.ID, now)
	})
	return c, err
}
func (s *Store) Confirm(ctx context.Context, userID, id, profileID string, now time.Time) (domain.SocialProfile, error) {
	var profile domain.SocialProfile
	err := s.transaction(ctx, func(tx pgx.Tx) error {
		// All owner operations lock the user before the challenge to avoid lock inversions.
		if err := lockUser(ctx, tx, userID); err != nil {
			return err
		}
		c, err := scanChallenge(tx.QueryRow(ctx, `SELECT `+challengeColumns+` FROM app.verification_challenges WHERE id=$1 AND user_id=$2 FOR UPDATE`, id, userID))
		if err != nil {
			return err
		}
		if err = c.Confirm(userID, now); err != nil {
			return err
		}
		profile = domain.SocialProfile{ID: profileID, UserID: userID, Platform: "instagram", Handle: c.ObservedHandle, Method: c.Method, VerifiedAt: now}
		if _, err = tx.Exec(ctx, `INSERT INTO app.social_profiles(id,user_id,platform,handle,method,verified_at,remote_account_id,remote_user_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, profile.ID, profile.UserID, profile.Platform, profile.Handle, profile.Method, profile.VerifiedAt, c.ObservedAccountID, c.ObservedSenderID); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE app.verification_challenges SET status='verified',consumed_at=$2 WHERE id=$1`, id, now); err != nil {
			return err
		}
		return audit(ctx, tx, userID, "instagram.profile_linked", profileID, now)
	})
	return profile, err
}
func (s *Store) Reject(ctx context.Context, actor, id, reason string, now time.Time) error {
	return s.transaction(ctx, func(tx pgx.Tx) error {
		c, err := scanChallenge(tx.QueryRow(ctx, `SELECT `+challengeColumns+` FROM app.verification_challenges WHERE id=$1 FOR UPDATE`, id))
		if err != nil {
			return err
		}
		if err = c.Reject(now); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE app.verification_challenges SET status='rejected',reviewed_by=$2,review_reference=$3 WHERE id=$1`, id, actor, reason); err != nil {
			return err
		}
		return audit(ctx, tx, actor, "instagram.challenge_rejected", id, now)
	})
}
func (s *Store) ActiveProfile(ctx context.Context, userID string) (domain.SocialProfile, error) {
	var p domain.SocialProfile
	err := s.pool.QueryRow(ctx, `SELECT id,user_id,platform,handle,method,verified_at FROM app.social_profiles WHERE user_id=$1 AND revoked_at IS NULL`, userID).Scan(&p.ID, &p.UserID, &p.Platform, &p.Handle, &p.Method, &p.VerifiedAt)
	return p, translate(err)
}
func (s *Store) RevokeProfile(ctx context.Context, userID, id string, now time.Time) error {
	return s.transaction(ctx, func(tx pgx.Tx) error {
		if err := lockUser(ctx, tx, userID); err != nil {
			return err
		}
		r, err := tx.Exec(ctx, `UPDATE app.social_profiles SET revoked_at=$3 WHERE id=$1 AND user_id=$2 AND revoked_at IS NULL`, id, userID, now)
		if err != nil {
			return err
		}
		if r.RowsAffected() != 1 {
			return domain.ErrNotFound
		}
		return audit(ctx, tx, userID, "instagram.profile_revoked", id, now)
	})
}
func (s *Store) PendingChallenges(ctx context.Context, limit int) ([]domain.Challenge, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+challengeColumns+` FROM app.verification_challenges WHERE status IN ('pending','awaiting_confirmation') AND expires_at>now() ORDER BY created_at,id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.Challenge, 0)
	for rows.Next() {
		c, err := scanChallenge(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}
