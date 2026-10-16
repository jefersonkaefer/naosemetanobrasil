package application_test

import (
	"context"
	"errors"
	"naosemeta.com/backend/internal/application"
	"naosemeta.com/backend/internal/domain"
	"testing"
	"time"
)

type incomingStore struct {
	seen                         bool
	candidateErr, observeErr     error
	candidateCalls, observations int
	rawText                      string
}

func (s *incomingStore) InstagramReceipt(context.Context, string, string) (bool, error) {
	return s.seen, nil
}
func (s *incomingStore) MessageChallenge(context.Context, string, time.Time) (domain.Challenge, error) {
	s.candidateCalls++
	return domain.Challenge{}, s.candidateErr
}
func (s *incomingStore) ObserveInstagram(_ context.Context, m domain.InstagramMessage, hash, handle string, _ time.Time) error {
	s.observations++
	s.rawText = m.Text
	if hash != "digest:challenge:NSM-abcdefghijklmnopqrstuv" || handle != "pessoa" {
		panic("unexpected proof")
	}
	return s.observeErr
}

type profileLookup struct {
	calls int
	err   error
}

func (p *profileLookup) Handle(context.Context, string) (string, error) {
	p.calls++
	return "pessoa", p.err
}

type incomingSecrets struct{}

func (incomingSecrets) NewID() string          { return "id" }
func (incomingSecrets) Token(int) string       { return "token" }
func (incomingSecrets) Digest(v string) string { return "digest:" + v }

type incomingClock struct{}

func (incomingClock) Now() time.Time { return time.Now() }

func TestIncomingMessageUsesExactCodesAndRetriesOnlyTransientErrors(t *testing.T) {
	for _, tc := range []struct {
		name, text                           string
		seen                                 bool
		candidateErr, profileErr, observeErr error
		lookups, observations                int
		fails                                bool
	}{
		{name: "valid", text: " NSM-abcdefghijklmnopqrstuv\n", lookups: 1, observations: 1},
		{name: "generic message", text: "oi"},
		{name: "embedded code", text: "quote: NSM-abcdefghijklmnopqrstuv"},
		{name: "duplicate", text: "NSM-abcdefghijklmnopqrstuv", seen: true},
		{name: "unknown code", text: "NSM-abcdefghijklmnopqrstuv", candidateErr: domain.ErrNotFound},
		{name: "lookup outage", text: "NSM-abcdefghijklmnopqrstuv", profileErr: errors.New("outage"), lookups: 1, fails: true},
		{name: "wrong sender", text: "NSM-abcdefghijklmnopqrstuv", observeErr: domain.ErrConflict, lookups: 1, observations: 1},
		{name: "cancelled concurrently", text: "NSM-abcdefghijklmnopqrstuv", observeErr: domain.ErrTransition, lookups: 1, observations: 1},
		{name: "database outage", text: "NSM-abcdefghijklmnopqrstuv", candidateErr: errors.New("outage"), fails: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &incomingStore{seen: tc.seen, candidateErr: tc.candidateErr, observeErr: tc.observeErr}
			profiles := &profileLookup{err: tc.profileErr}
			v := application.NewInstagramVerification(store, profiles, incomingSecrets{}, incomingClock{})
			err := v.Receive(context.Background(), domain.InstagramMessage{Text: tc.text})
			if (err != nil) != tc.fails || profiles.calls != tc.lookups || store.observations != tc.observations || store.rawText != "" {
				t.Fatalf("err=%v lookups=%d writes=%d raw=%q", err, profiles.calls, store.observations, store.rawText)
			}
		})
	}
}
