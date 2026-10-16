package domain_test

import (
	"naosemeta.com/backend/internal/domain"
	"testing"
	"time"
)

func TestInstagramProofCannotSkipSenderOrChallengeChecks(t *testing.T) {
	now := time.Now().UTC()
	for _, tc := range []struct {
		name, handle, sender string
		status               domain.ChallengeStatus
		sent, expires        time.Time
		ok                   bool
	}{
		{"valid", "pessoa", "456", domain.Pending, now, now.Add(time.Hour), true},
		{"wrong sender handle", "outra", "456", domain.Pending, now, now.Add(time.Hour), false},
		{"missing sender", "pessoa", "", domain.Pending, now, now.Add(time.Hour), false},
		{"replay before issuance", "pessoa", "456", domain.Pending, now.Add(-time.Hour), now.Add(time.Hour), false},
		{"expired", "pessoa", "456", domain.Pending, now, now, false},
		{"cancelled", "pessoa", "456", domain.Cancelled, now, now.Add(time.Hour), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := domain.Challenge{ExpectedHandle: "pessoa", Status: tc.status, CreatedAt: now, ExpiresAt: tc.expires}
			err := c.ObserveInstagram(tc.handle, "123", tc.sender, "mid", tc.sent, now)
			if (err == nil) != tc.ok {
				t.Fatal(err)
			}
			if tc.ok && (c.Status != domain.AwaitingConfirmation || c.Method != "instagram_webhook" || c.ReviewedBy != "" || c.ObservedSenderID != "456") {
				t.Fatal("invalid evidence state")
			}
		})
	}
}
