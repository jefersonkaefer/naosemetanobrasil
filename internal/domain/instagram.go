package domain

import (
	"regexp"
	"time"
)

var instagramIdentity = regexp.MustCompile(`^[0-9]{1,32}$`)

// InstagramMessage contains only the identity evidence needed for verification.
// Its text is processed transiently and is never persisted as a message archive.
type InstagramMessage struct {
	AccountID string
	SenderID  string
	MessageID string
	Text      string
	SentAt    time.Time
}

func (c *Challenge) ObserveInstagram(handle, accountID, senderID, messageID string, sentAt, now time.Time) error {
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
	if !instagramIdentity.MatchString(accountID) || !instagramIdentity.MatchString(senderID) || messageID == "" || len(messageID) > 512 || sentAt.Before(c.CreatedAt.Truncate(time.Millisecond)) || sentAt.After(now.Add(5*time.Minute)) {
		return ErrInvalid
	}
	c.ObservedHandle, c.ObservedAccountID, c.ObservedSenderID = normalized, accountID, senderID
	c.ReviewedBy, c.ReviewReference, c.Method = "", "", "instagram_webhook"
	c.Status = AwaitingConfirmation
	if c.SentAt == nil {
		c.SentAt = &sentAt
	}
	return nil
}
