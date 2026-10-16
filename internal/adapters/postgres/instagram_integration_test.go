//go:build integration

package postgres_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"naosemeta.com/backend/internal/adapters/httpapi"
	"naosemeta.com/backend/internal/adapters/instagram"
	"naosemeta.com/backend/internal/application"
	"naosemeta.com/backend/internal/domain"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type profileFunc func(context.Context, string) (string, error)

func (f profileFunc) Handle(ctx context.Context, id string) (string, error) { return f(ctx, id) }

func webhookPayload(t *testing.T, code, sender, mid string, now time.Time) string {
	t.Helper()
	value := map[string]any{"object": "instagram", "entry": []any{map[string]any{"id": "123456", "messaging": []any{map[string]any{"sender": map[string]string{"id": sender}, "recipient": map[string]string{"id": "123456"}, "timestamp": now.UnixMilli(), "message": map[string]string{"mid": mid, "text": code}}}}}}
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func postWebhook(h http.Handler, body, secret string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/v1/webhooks/instagram", strings.NewReader(body))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	r.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func incomingHandler(f *fixture, profiles application.InstagramProfiles) http.Handler {
	v := application.NewInstagramVerification(f.store, profiles, f.secrets, f.clock)
	webhook := instagram.NewWebhook("app-secret", "verify-token", "123456", v)
	return httpapi.New(f.auth, f.verification, f.limiter, f.clock, f.secrets, httpapi.Options{Origin: "https://platform.test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Ready: f.store.Ready, InstagramWebhook: webhook, InstagramMode: "webhook"})
}

func TestAutomaticInstagramFlowIsSignedIdempotentAndPreservesRemoteIdentity(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	member := f.member(t)
	handle := "p_" + strings.ReplaceAll(f.secrets.NewID(), "-", "")[:20]
	issued, err := f.verification.Issue(ctx, member.User.ID, handle)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	h := incomingHandler(f, profileFunc(func(_ context.Context, id string) (string, error) {
		calls.Add(1)
		if id == "999" {
			return "different_profile", nil
		}
		return handle, nil
	}))
	body := webhookPayload(t, issued.Code, "456", f.secrets.NewID(), f.clock.Now())
	if w := postWebhook(h, body, "wrong-secret"); w.Code != 403 {
		t.Fatal("unsigned proof accepted", w.Code)
	}
	wrong := webhookPayload(t, issued.Code, "999", f.secrets.NewID(), f.clock.Now())
	if w := postWebhook(h, wrong, "app-secret"); w.Code != 200 {
		t.Fatal(w.Code)
	}
	c, err := f.verification.Get(ctx, member.User.ID, issued.Challenge.ID)
	if err != nil || c.Status != domain.Pending {
		t.Fatal("wrong sender changed challenge", err, c.Status)
	}
	if w := postWebhook(h, body, "app-secret"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	c, err = f.verification.Get(ctx, member.User.ID, issued.Challenge.ID)
	if err != nil || c.Status != domain.AwaitingConfirmation || c.ReviewedBy != "" || c.Method != "instagram_webhook" || c.SentAt == nil {
		t.Fatal("automatic proof not recorded", err, c)
	}
	before := calls.Load()
	if err = f.verification.Sent(ctx, member.User.ID, c.ID); err != nil {
		t.Fatal("sent click racing with webhook failed", err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if w := postWebhook(h, body, "app-secret"); w.Code != 200 {
				t.Errorf("redelivery failed: %d", w.Code)
			}
		})
	}
	wg.Wait()
	if calls.Load() != before {
		t.Fatal("duplicate message triggered a new profile request")
	}
	var receipts, audits int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM app.instagram_message_receipts WHERE challenge_id=$1`, c.ID).Scan(&receipts); err != nil {
		t.Fatal(err)
	}
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM app.audit_events WHERE entity_id=$1 AND action='instagram.message_verified_automatically'`, c.ID).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if receipts != 1 || audits != 1 {
		t.Fatal("non-idempotent transaction", receipts, audits)
	}
	p, err := f.verification.Confirm(ctx, member.User.ID, c.ID)
	if err != nil || p.Method != "instagram_webhook" {
		t.Fatal("confirmation failed", err, p)
	}
	var remote string
	if err = f.pool.QueryRow(ctx, `SELECT remote_user_id FROM app.social_profiles WHERE id=$1`, p.ID).Scan(&remote); err != nil || remote != "456" {
		t.Fatal("remote identity lost", err, remote)
	}
	if w := postWebhook(h, body, "app-secret"); w.Code != 200 {
		t.Fatal("redelivery after confirmation failed", w.Code)
	}
	if _, err = f.verification.Confirm(ctx, member.User.ID, c.ID); !errors.Is(err, domain.ErrTransition) {
		t.Fatal("challenge reused", err)
	}
	// A rename must not let the same scoped Instagram identity acquire a
	// second active owner under a different handle.
	other := f.member(t)
	newHandle := "r_" + strings.ReplaceAll(f.secrets.NewID(), "-", "")[:20]
	second, err := f.verification.Issue(ctx, other.User.ID, newHandle)
	if err != nil {
		t.Fatal(err)
	}
	h = incomingHandler(f, profileFunc(func(context.Context, string) (string, error) { return newHandle, nil }))
	if w := postWebhook(h, webhookPayload(t, second.Code, "456", f.secrets.NewID(), f.clock.Now()), "app-secret"); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if _, err = f.verification.Confirm(ctx, other.User.ID, second.Challenge.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("same remote identity linked twice", err)
	}
	if err = f.verification.Revoke(ctx, member.User.ID, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.verification.Confirm(ctx, other.User.ID, second.Challenge.ID); err != nil {
		t.Fatal("revoked identity cannot be linked again", err)
	}
}

func TestConcurrentFirstInstagramDeliveriesCommitOneReceipt(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	member := f.member(t)
	handle := "p_" + strings.ReplaceAll(f.secrets.NewID(), "-", "")[:20]
	issued, err := f.verification.Issue(ctx, member.User.ID, handle)
	if err != nil {
		t.Fatal(err)
	}
	h := incomingHandler(f, profileFunc(func(context.Context, string) (string, error) { return handle, nil }))
	body := webhookPayload(t, issued.Code, "789", f.secrets.NewID(), f.clock.Now())
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			if w := postWebhook(h, body, "app-secret"); w.Code != 200 {
				t.Errorf("concurrent delivery: %d", w.Code)
			}
		})
	}
	wg.Wait()
	var count int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM app.instagram_message_receipts WHERE challenge_id=$1`, issued.Challenge.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate receipt", err, count)
	}
	if _, err = f.verification.Confirm(ctx, member.User.ID, issued.Challenge.ID); err != nil {
		t.Fatal(err)
	}
}

func TestAutomaticInstagramRetriesOutagesAndIgnoresExpiredOrCancelledCodes(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	member := f.member(t)
	handle := "p_" + strings.ReplaceAll(f.secrets.NewID(), "-", "")[:20]
	issued, err := f.verification.Issue(ctx, member.User.ID, handle)
	if err != nil {
		t.Fatal(err)
	}
	fail := true
	calls := 0
	h := incomingHandler(f, profileFunc(func(context.Context, string) (string, error) {
		calls++
		if fail {
			return "", errors.New("Meta temporarily unavailable")
		}
		return handle, nil
	}))
	body := webhookPayload(t, issued.Code, "567", f.secrets.NewID(), f.clock.Now())
	if w := postWebhook(h, body, "app-secret"); w.Code != 503 {
		t.Fatal("outage acknowledged", w.Code)
	}
	c, err := f.verification.Get(ctx, member.User.ID, issued.Challenge.ID)
	if err != nil || c.Status != domain.Pending {
		t.Fatal("outage changed state", err, c.Status)
	}
	fail = false
	if w := postWebhook(h, body, "app-secret"); w.Code != 200 {
		t.Fatal("retry failed", w.Code)
	}
	// Issuing a replacement cancels the previous proof, including proofs that
	// were awaiting final confirmation.
	replacement, err := f.verification.Issue(ctx, member.User.ID, handle)
	if err != nil {
		t.Fatal(err)
	}
	before := calls
	if w := postWebhook(h, webhookPayload(t, issued.Code, "567", f.secrets.NewID(), f.clock.Now()), "app-secret"); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if calls != before {
		t.Fatal("cancelled code caused profile request")
	}
	f.clock.now = f.clock.now.Add(25 * time.Hour)
	if w := postWebhook(h, webhookPayload(t, replacement.Code, "567", f.secrets.NewID(), f.clock.Now()), "app-secret"); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if calls != before {
		t.Fatal("expired code caused profile request")
	}
}
