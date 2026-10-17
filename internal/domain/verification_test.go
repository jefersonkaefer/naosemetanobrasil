package domain_test

import (
	"errors"
	"testing"
	"time"

	"naosemeta.com/backend/internal/domain"
)

func challenge() domain.Challenge {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	return domain.Challenge{ID: "challenge", UserID: "owner", ExpectedHandle: "pessoa", Status: domain.Pending, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)}
}
func TestVerificationRequiresObservationAndOwnerConfirmation(t *testing.T) {
	c := challenge()
	now := c.CreatedAt.Add(time.Minute)
	if err := c.Confirm("owner", now); !errors.Is(err, domain.ErrTransition) {
		t.Fatalf("confirmation without observation: %v", err)
	}
	if err := c.MarkSent(now); err != nil {
		t.Fatal(err)
	}
	if c.Status != domain.Pending {
		t.Fatal("reporting a sent message verified the account")
	}
	if err := c.Observe("@OUTRA", "admin", "inbox reference", now); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("wrong sender: %v", err)
	}
	if err := c.Observe("@Pessoa", "admin", "inbox reference", now); err != nil {
		t.Fatal(err)
	}
	if err := c.Confirm("intruder", now); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("wrong owner: %v", err)
	}
	if c.Status != domain.AwaitingConfirmation {
		t.Fatal("wrong owner modified challenge")
	}
	if err := c.Confirm("owner", now); err != nil {
		t.Fatal(err)
	}
	if err := c.Confirm("owner", now); !errors.Is(err, domain.ErrTransition) {
		t.Fatalf("code reuse: %v", err)
	}
}
func TestExpirationAtBoundary(t *testing.T) {
	for _, operation := range []string{"sent", "observe", "confirm"} {
		t.Run(operation, func(t *testing.T) {
			c := challenge()
			var err error
			switch operation {
			case "sent":
				err = c.MarkSent(c.ExpiresAt)
			case "observe":
				err = c.Observe("pessoa", "admin", "reference", c.ExpiresAt)
			case "confirm":
				if err = c.Observe("pessoa", "admin", "reference", c.CreatedAt); err != nil {
					t.Fatal(err)
				}
				err = c.Confirm("owner", c.ExpiresAt)
			}
			if !errors.Is(err, domain.ErrExpired) {
				t.Fatalf("expected expired: %v", err)
			}
		})
	}
}
func TestRejectedCancelledAndVerifiedChallengesAreTerminal(t *testing.T) {
	for _, state := range []domain.ChallengeStatus{domain.Rejected, domain.Cancelled, domain.Verified} {
		c := challenge()
		c.Status = state
		if err := c.Observe("pessoa", "admin", "reference", c.CreatedAt); !errors.Is(err, domain.ErrTransition) {
			t.Errorf("%s: %v", state, err)
		}
	}
}
func TestAdministratorCannotObserveOwnProfile(t *testing.T) {
	c := challenge()
	if err := c.Observe("pessoa", "owner", "reference", c.CreatedAt); !errors.Is(err, domain.ErrSelfReview) {
		t.Fatal(err)
	}
	if c.Status != domain.Pending || c.ReviewedBy != "" || c.ObservedHandle != "" {
		t.Fatal("self-review modified the challenge")
	}
}
func TestNormalizeHandleRejectsURLsAndLookalikes(t *testing.T) {
	for _, value := range []string{"", "https://instagram.com/alguem", "a..b", ".nome", "nome.", "pessоa", "a/b", "a b"} {
		if _, err := domain.NormalizeHandle(value); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("accepted %q", value)
		}
	}
	got, err := domain.NormalizeHandle(" @Pessoa_1 ")
	if err != nil || got != "pessoa_1" {
		t.Fatalf("%q %v", got, err)
	}
}
