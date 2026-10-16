// Package application defines use cases and their outgoing ports.
// It has no dependency on HTTP, SQL drivers or infrastructure providers.
package application

import (
	"context"
	"time"

	"naosemeta.com/backend/internal/domain"
)

type Clock interface{ Now() time.Time }
type Secrets interface {
	NewID() string
	Token(int) string
	Digest(string) string
}
type Passwords interface {
	Hash(context.Context, string) (string, error)
	Verify(context.Context, string, string) (bool, error)
	DummyHash() string
}
type SecondFactor interface {
	Verify(encrypted []byte, code string, now time.Time) (int64, error)
}
type AccountStore interface {
	CreateAccount(context.Context, domain.Credential, []string) error
	Credentials(context.Context, string) (domain.Credential, error)
	Recover(context.Context, string, string, string, time.Time) error
}
type SessionStore interface {
	// CreateSession must compare the checked password hash and consume any TOTP
	// counter atomically, preventing login races with recovery and code reuse.
	CreateSession(context.Context, domain.Session, string, *int64) error
	SessionUser(context.Context, string, time.Time) (domain.User, error)
	RevokeSession(context.Context, string) error
}
type VerificationStore interface {
	// IssueChallenge atomically cancels the owner's previous pending challenge.
	IssueChallenge(context.Context, domain.Challenge) error
	Challenge(context.Context, string, string) (domain.Challenge, error)
	MarkSent(context.Context, string, string, time.Time) error
	Observe(context.Context, string, string, string, string, time.Time) (domain.Challenge, error)
	// Confirm must lock current state and commit profile uniqueness, challenge
	// consumption and audit together; conflicts must leave no partial writes.
	Confirm(context.Context, string, string, string, time.Time) (domain.SocialProfile, error)
	Reject(context.Context, string, string, string, time.Time) error
	ActiveProfile(context.Context, string) (domain.SocialProfile, error)
	RevokeProfile(context.Context, string, string, time.Time) error
	PendingChallenges(context.Context, int) ([]domain.Challenge, error)
}
type RateLimiter interface {
	Allow(context.Context, string, int, time.Duration, time.Time) error
}

type InstagramProfiles interface {
	Handle(context.Context, string) (string, error)
}

type InstagramMessageStore interface {
	InstagramReceipt(context.Context, string, string) (bool, error)
	MessageChallenge(context.Context, string, time.Time) (domain.Challenge, error)
	ObserveInstagram(context.Context, domain.InstagramMessage, string, string, time.Time) error
}
