//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"naosemeta.com/backend/internal/domain"
)

func TestOwnReviewIsDeniedWithoutChangingPendingRequest(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	actor, _ := f.admin(t)
	issued, err := f.verification.Issue(ctx, actor.ID, "own_profile")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.verification.Observe(ctx, actor, issued.Code, "own_profile", "synthetic inbox reference"); !errors.Is(err, domain.ErrSelfReview) {
		t.Fatal("missing explicit self-review rejection", err)
	}
	request, err := f.verification.Get(ctx, actor.ID, issued.Challenge.ID)
	if err != nil || request.Status != domain.Pending || request.ReviewedBy != "" || request.ObservedHandle != "" {
		t.Fatal("denied review changed request", err)
	}
}
