package application

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"naosemeta.com/backend/internal/domain"
)

type Auth struct {
	accounts     AccountStore
	sessions     SessionStore
	passwords    Passwords
	secondFactor SecondFactor
	secrets      Secrets
	clock        Clock
}

func NewAuth(accounts AccountStore, sessions SessionStore, passwords Passwords, secondFactor SecondFactor, secrets Secrets, clock Clock) *Auth {
	return &Auth{accounts, sessions, passwords, secondFactor, secrets, clock}
}

type Registration struct {
	User          domain.User
	RecoveryCodes []string
}

func (a *Auth) Register(ctx context.Context, username, password, displayName string) (Registration, error) {
	username, err := domain.NormalizeUsername(username)
	if err != nil {
		return Registration{}, err
	}
	if err = domain.ValidatePassword(password); err != nil {
		return Registration{}, err
	}
	displayName = strings.TrimSpace(displayName)
	if !utf8.ValidString(displayName) || utf8.RuneCountInString(displayName) > 80 {
		return Registration{}, domain.ErrInvalid
	}
	hash, err := a.passwords.Hash(ctx, password)
	if err != nil {
		return Registration{}, err
	}
	user := domain.User{ID: a.secrets.NewID(), Username: username, DisplayName: displayName, Role: "member", CreatedAt: a.clock.Now()}
	codes, digests := make([]string, 10), make([]string, 10)
	for i := range codes {
		codes[i] = "NSR-" + a.secrets.Token(16)
		digests[i] = a.secrets.Digest("recovery:" + codes[i])
	}
	if err = a.accounts.CreateAccount(ctx, domain.Credential{User: user, PasswordHash: hash}, digests); err != nil {
		return Registration{}, err
	}
	return Registration{user, codes}, nil
}

type Login struct {
	User      domain.User
	Token     string
	CSRFToken string
	ExpiresAt time.Time
}

func (a *Auth) Login(ctx context.Context, username, password, otp string) (Login, error) {
	if len(password) > 128 || len(username) > 100 || len(otp) > 10 {
		return Login{}, domain.ErrUnauthenticated
	}
	username = strings.ToLower(strings.TrimSpace(username))
	credential, err := a.accounts.Credentials(ctx, username)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return Login{}, err
	}
	missing := errors.Is(err, domain.ErrNotFound)
	hash := credential.PasswordHash
	if missing {
		hash = a.passwords.DummyHash()
	}
	valid, err := a.passwords.Verify(ctx, password, hash)
	if err != nil {
		return Login{}, err
	}
	if !valid || missing {
		return Login{}, domain.ErrUnauthenticated
	}
	now := a.clock.Now()
	var step *int64
	if credential.User.IsAdmin() {
		counter, err := a.secondFactor.Verify(credential.TOTPEncrypted, otp, now)
		if err != nil {
			return Login{}, domain.ErrUnauthenticated
		}
		step = &counter
	}
	token := a.secrets.Token(32)
	session := domain.Session{ID: a.secrets.NewID(), UserID: credential.User.ID, TokenHash: a.secrets.Digest("session:" + token), ExpiresAt: now.Add(7 * 24 * time.Hour)}
	if err = a.sessions.CreateSession(ctx, session, credential.PasswordHash, step); err != nil {
		return Login{}, err
	}
	return Login{credential.User, token, a.CSRF(token), session.ExpiresAt}, nil
}

func (a *Auth) Authenticate(ctx context.Context, token string) (domain.User, error) {
	if len(token) != 43 {
		return domain.User{}, domain.ErrUnauthenticated
	}
	return a.sessions.SessionUser(ctx, a.secrets.Digest("session:"+token), a.clock.Now())
}

func (a *Auth) CSRF(token string) string { return a.secrets.Digest("csrf:" + token) }

func (a *Auth) Logout(ctx context.Context, token string) error {
	return a.sessions.RevokeSession(ctx, a.secrets.Digest("session:"+token))
}

func (a *Auth) Recover(ctx context.Context, username, code, password string) error {
	username, err := domain.NormalizeUsername(username)
	if err != nil {
		return domain.ErrUnauthenticated
	}
	if len(code) > 100 {
		return domain.ErrUnauthenticated
	}
	if err = domain.ValidatePassword(password); err != nil {
		return err
	}
	hash, err := a.passwords.Hash(ctx, password)
	if err != nil {
		return err
	}
	return a.accounts.Recover(ctx, username, a.secrets.Digest("recovery:"+code), hash, a.clock.Now())
}
