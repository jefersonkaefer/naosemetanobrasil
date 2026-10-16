package domain

import "time"

type ChallengeStatus string

const (
	Pending              ChallengeStatus = "pending"
	AwaitingConfirmation ChallengeStatus = "awaiting_confirmation"
	Verified             ChallengeStatus = "verified"
	Expired              ChallengeStatus = "expired"
	Rejected             ChallengeStatus = "rejected"
	Cancelled            ChallengeStatus = "cancelled"
)

type Challenge struct {
	ID                string
	UserID            string
	ExpectedHandle    string
	CodeHash          string
	Status            ChallengeStatus
	CreatedAt         time.Time
	ExpiresAt         time.Time
	SentAt            *time.Time
	ObservedHandle    string
	ReviewedBy        string
	ReviewReference   string
	Method            string
	ObservedAccountID string
	ObservedSenderID  string
}

func (c Challenge) EffectiveStatus(now time.Time) ChallengeStatus {
	if (c.Status == Pending || c.Status == AwaitingConfirmation) && !now.Before(c.ExpiresAt) {
		return Expired
	}
	return c.Status
}

func (c *Challenge) MarkSent(now time.Time) error {
	if c.EffectiveStatus(now) == Expired {
		return ErrExpired
	}
	// The webhook may arrive between the user sending the DM and clicking
	// "Já enviei". Acknowledging that click must not regress verified evidence.
	if c.Status == AwaitingConfirmation {
		return nil
	}
	if c.Status != Pending {
		return ErrTransition
	}
	if c.SentAt == nil {
		c.SentAt = &now
	}
	return nil
}

func (c *Challenge) Observe(handle, reviewerID, reference string, now time.Time) error {
	if c.EffectiveStatus(now) == Expired {
		return ErrExpired
	}
	if c.Status != Pending {
		return ErrTransition
	}
	normalized, err := NormalizeHandle(handle)
	if err != nil {
		return err
	}
	if normalized != c.ExpectedHandle {
		return ErrConflict
	}
	if reviewerID == "" || reference == "" || len(reference) > 200 || reviewerID == c.UserID {
		return ErrInvalid
	}
	c.ObservedHandle, c.ReviewedBy, c.ReviewReference = normalized, reviewerID, reference
	c.Method = "manual_dm"
	c.Status = AwaitingConfirmation
	return nil
}

func (c *Challenge) Confirm(userID string, now time.Time) error {
	if c.UserID != userID {
		return ErrNotFound
	}
	if c.EffectiveStatus(now) == Expired {
		return ErrExpired
	}
	if c.Status != AwaitingConfirmation || c.ObservedHandle != c.ExpectedHandle {
		return ErrTransition
	}
	c.Status = Verified
	return nil
}

func (c *Challenge) Reject(now time.Time) error {
	if c.EffectiveStatus(now) == Expired {
		return ErrExpired
	}
	if c.Status != Pending && c.Status != AwaitingConfirmation {
		return ErrTransition
	}
	c.Status = Rejected
	return nil
}
