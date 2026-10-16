package application

import (
	"context"
	"time"

	"naosemeta.com/backend/internal/domain"
)

type Verification struct {
	store          VerificationStore
	limiter        RateLimiter
	secrets        Secrets
	clock          Clock
	officialHandle string
}

func NewVerification(store VerificationStore, limiter RateLimiter, secrets Secrets, clock Clock, officialHandle string) *Verification {
	return &Verification{store, limiter, secrets, clock, officialHandle}
}

type IssuedChallenge struct {
	Challenge      domain.Challenge
	Code           string
	OfficialHandle string
}

func (v *Verification) Issue(ctx context.Context, userID, handle string) (IssuedChallenge, error) {
	handle, err := domain.NormalizeHandle(handle)
	if err != nil {
		return IssuedChallenge{}, err
	}
	now := v.clock.Now()
	if err = v.limiter.Allow(ctx, "challenge-hour:"+userID, 3, time.Hour, now); err != nil {
		return IssuedChallenge{}, err
	}
	if err = v.limiter.Allow(ctx, "challenge-day:"+userID, 10, 24*time.Hour, now); err != nil {
		return IssuedChallenge{}, err
	}
	code := "NSM-" + v.secrets.Token(16)
	c := domain.Challenge{ID: v.secrets.NewID(), UserID: userID, ExpectedHandle: handle, CodeHash: v.secrets.Digest("challenge:" + code), Status: domain.Pending, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)}
	if err = v.store.IssueChallenge(ctx, c); err != nil {
		return IssuedChallenge{}, err
	}
	return IssuedChallenge{c, code, v.officialHandle}, nil
}

func (v *Verification) Get(ctx context.Context, userID, id string) (domain.Challenge, error) {
	c, err := v.store.Challenge(ctx, userID, id)
	if err == nil {
		c.Status = c.EffectiveStatus(v.clock.Now())
	}
	return c, err
}

func (v *Verification) Sent(ctx context.Context, userID, id string) error {
	return v.store.MarkSent(ctx, userID, id, v.clock.Now())
}

func (v *Verification) Observe(ctx context.Context, actor domain.User, code, handle, reference string) (domain.Challenge, error) {
	if !actor.IsAdmin() {
		return domain.Challenge{}, domain.ErrForbidden
	}
	if len(code) > 100 {
		return domain.Challenge{}, domain.ErrInvalid
	}
	return v.store.Observe(ctx, actor.ID, v.secrets.Digest("challenge:"+code), handle, reference, v.clock.Now())
}

func (v *Verification) Confirm(ctx context.Context, userID, id string) (domain.SocialProfile, error) {
	return v.store.Confirm(ctx, userID, id, v.secrets.NewID(), v.clock.Now())
}

func (v *Verification) Reject(ctx context.Context, actor domain.User, id, reason string) error {
	if !actor.IsAdmin() {
		return domain.ErrForbidden
	}
	if reason == "" || len(reason) > 300 {
		return domain.ErrInvalid
	}
	return v.store.Reject(ctx, actor.ID, id, reason, v.clock.Now())
}

func (v *Verification) Profile(ctx context.Context, userID string) (domain.SocialProfile, error) {
	return v.store.ActiveProfile(ctx, userID)
}

func (v *Verification) Revoke(ctx context.Context, userID, id string) error {
	return v.store.RevokeProfile(ctx, userID, id, v.clock.Now())
}

func (v *Verification) Pending(ctx context.Context, actor domain.User) ([]domain.Challenge, error) {
	if !actor.IsAdmin() {
		return nil, domain.ErrForbidden
	}
	return v.store.PendingChallenges(ctx, 50)
}
