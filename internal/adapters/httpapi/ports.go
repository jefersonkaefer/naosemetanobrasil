package httpapi

import (
	"context"

	"naosemeta.com/backend/internal/application"
	"naosemeta.com/backend/internal/domain"
)

// Inbound adapter contracts expose only the operations used by HTTP.
type AuthUseCases interface {
	Register(context.Context, string, string, string) (application.Registration, error)
	Login(context.Context, string, string, string) (application.Login, error)
	Authenticate(context.Context, string) (domain.User, error)
	CSRF(string) string
	Logout(context.Context, string) error
	Recover(context.Context, string, string, string) error
}
type VerificationUseCases interface {
	Issue(context.Context, string, string) (application.IssuedChallenge, error)
	Get(context.Context, string, string) (domain.Challenge, error)
	Sent(context.Context, string, string) error
	Observe(context.Context, domain.User, string, string, string) (domain.Challenge, error)
	Confirm(context.Context, string, string) (domain.SocialProfile, error)
	Reject(context.Context, domain.User, string, string) error
	Profile(context.Context, string) (domain.SocialProfile, error)
	Revoke(context.Context, string, string) error
	Pending(context.Context, domain.User) ([]domain.Challenge, error)
}
