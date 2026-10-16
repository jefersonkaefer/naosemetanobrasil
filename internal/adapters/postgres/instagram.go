package postgres

import (
	"context"
	"github.com/jackc/pgx/v5"
	"naosemeta.com/backend/internal/domain"
	"time"
)

func (s *Store) InstagramReceipt(ctx context.Context, accountID, messageID string) (bool, error) {
	var found bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.instagram_message_receipts WHERE account_id=$1 AND message_id=$2)`, accountID, messageID).Scan(&found)
	return found, err
}

func (s *Store) MessageChallenge(ctx context.Context, hash string, now time.Time) (domain.Challenge, error) {
	return scanChallenge(s.pool.QueryRow(ctx, `SELECT `+challengeColumns+` FROM app.verification_challenges WHERE code_hash=$1 AND status='pending' AND expires_at>$2`, hash, now))
}

func (s *Store) ObserveInstagram(ctx context.Context, message domain.InstagramMessage, hash, handle string, now time.Time) error {
	return s.transaction(ctx, func(tx pgx.Tx) error {
		// Owner lock follows the same order as Issue and Confirm. Re-read the
		// challenge after the lock, so cancellation cannot race past inspection.
		var owner string
		if err := tx.QueryRow(ctx, `SELECT user_id FROM app.verification_challenges WHERE code_hash=$1`, hash).Scan(&owner); err != nil {
			return translate(err)
		}
		if err := lockUser(ctx, tx, owner); err != nil {
			return err
		}
		c, err := scanChallenge(tx.QueryRow(ctx, `SELECT `+challengeColumns+` FROM app.verification_challenges WHERE code_hash=$1 FOR UPDATE`, hash))
		if err != nil {
			return err
		}
		var seen bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.instagram_message_receipts WHERE account_id=$1 AND message_id=$2)`, message.AccountID, message.MessageID).Scan(&seen); err != nil {
			return err
		}
		if seen {
			return nil
		}
		if err = c.ObserveInstagram(handle, message.AccountID, message.SenderID, message.MessageID, message.SentAt, now); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO app.instagram_message_receipts(account_id,message_id,challenge_id,received_at) VALUES($1,$2,$3,$4)`, message.AccountID, message.MessageID, c.ID, now); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE app.verification_challenges SET status=$2,observed_handle=$3,reviewed_by=NULL,review_reference='',method=$4,observed_account_id=$5,observed_sender_id=$6,sent_at=$7 WHERE id=$1`, c.ID, c.Status, c.ObservedHandle, c.Method, c.ObservedAccountID, c.ObservedSenderID, c.SentAt); err != nil {
			return err
		}
		// The owner is the actor for this user-initiated proof. No synthetic admin
		// identity or fake manual reviewer is created for webhook processing.
		return audit(ctx, tx, owner, "instagram.message_verified_automatically", c.ID, now)
	})
}
