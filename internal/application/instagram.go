package application

import (
	"context"
	"errors"
	"naosemeta.com/backend/internal/domain"
	"regexp"
	"strings"
)

// A pasted code must be the entire message. URLs, quotes, attachments and
// messages containing multiple codes cannot accidentally link an account.
var instagramCode = regexp.MustCompile(`^NSM-[A-Za-z0-9_-]{22}$`)

type InstagramVerification struct {
	store    InstagramMessageStore
	profiles InstagramProfiles
	secrets  Secrets
	clock    Clock
}

func NewInstagramVerification(store InstagramMessageStore, profiles InstagramProfiles, secrets Secrets, clock Clock) *InstagramVerification {
	return &InstagramVerification{store, profiles, secrets, clock}
}

func (v *InstagramVerification) Receive(ctx context.Context, message domain.InstagramMessage) error {
	code := strings.TrimSpace(message.Text)
	if !instagramCode.MatchString(code) {
		return nil
	}
	seen, err := v.store.InstagramReceipt(ctx, message.AccountID, message.MessageID)
	if err != nil || seen {
		return err
	}
	hash := v.secrets.Digest("challenge:" + code)
	if _, err = v.store.MessageChallenge(ctx, hash, v.clock.Now()); err != nil {
		if ignoredInstagramError(err) {
			return nil
		}
		return err
	}
	handle, err := v.profiles.Handle(ctx, message.SenderID)
	if err != nil {
		return err
	}
	message.Text = ""
	err = v.store.ObserveInstagram(ctx, message, hash, handle, v.clock.Now())
	if ignoredInstagramError(err) {
		return nil
	}
	return err
}

func ignoredInstagramError(err error) bool {
	return errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrExpired) || errors.Is(err, domain.ErrTransition) || errors.Is(err, domain.ErrConflict) || errors.Is(err, domain.ErrInvalid) || errors.Is(err, domain.ErrForbidden)
}
