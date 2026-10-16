//go:build integration

package postgres_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pquerna/otp/totp"
	"naosemeta.com/backend/internal/adapters/httpapi"
	"naosemeta.com/backend/internal/adapters/postgres"
	"naosemeta.com/backend/internal/adapters/security"
	"naosemeta.com/backend/internal/application"
	"naosemeta.com/backend/internal/domain"
	"naosemeta.com/backend/migrations"
)

type fixedClock struct{ now time.Time }

func (c *fixedClock) Now() time.Time { return c.now }

type fixture struct {
	pool         *pgxpool.Pool
	store        *postgres.Store
	secrets      *security.Secrets
	passwords    *security.Passwords
	second       *security.SecondFactor
	clock        *fixedClock
	limiter      *postgres.Limiter
	auth         *application.Auth
	verification *application.Verification
}

func setup(t *testing.T) *fixture {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Fatal("integration tests require TEST_DATABASE_URL pointing to a disposable local *_test database")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.Hostname() != "::1") || !strings.HasSuffix(u.Path, "_test") {
		t.Fatal("refusing nonlocal or non-test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err = migrations.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	// Verify rerunning migrations is safe.
	if err = migrations.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	passwords, err := security.NewPasswords()
	if err != nil {
		t.Fatal(err)
	}
	second, err := security.NewSecondFactor(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{pool: pool, store: postgres.New(pool), secrets: security.NewSecrets("integration-test-pepper"), passwords: passwords, second: second, clock: &fixedClock{time.Now().UTC()}}
	f.limiter = postgres.NewLimiter(f.store, f.secrets)
	f.auth = application.NewAuth(f.store, f.store, passwords, second, f.secrets, f.clock)
	f.verification = application.NewVerification(f.store, f.limiter, f.secrets, f.clock, "projeto_test")
	return f
}
func (f *fixture) member(t *testing.T) application.Registration {
	t.Helper()
	r, err := f.auth.Register(context.Background(), "u_"+strings.ReplaceAll(f.secrets.NewID(), "-", "")[:20], "integration-password-long", "")
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func (f *fixture) admin(t *testing.T) (domain.User, string) {
	t.Helper()
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "Test", AccountName: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	hash, err := f.passwords.Hash(context.Background(), "admin-password-long")
	if err != nil {
		t.Fatal(err)
	}
	u := domain.User{ID: f.secrets.NewID(), Username: "a_" + strings.ReplaceAll(f.secrets.NewID(), "-", "")[:20], Role: "admin", CreatedAt: f.clock.Now()}
	if err = f.store.CreateAccount(context.Background(), domain.Credential{User: u, PasswordHash: hash, TOTPEncrypted: f.second.Seal(key.Secret())}, nil); err != nil {
		t.Fatal(err)
	}
	return u, key.Secret()
}
func TestManualDirectFlowOverHTTP(t *testing.T) {
	f := setup(t)
	admin, secret := f.admin(t)
	h := httpapi.New(f.auth, f.verification, f.limiter, f.clock, f.secrets, httpapi.Options{Origin: "http://localhost", Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Ready: f.pool.Ping})
	request := func(method, path string, body any, cookie *http.Cookie, csrf string) (*httptest.ResponseRecorder, map[string]json.RawMessage) {
		t.Helper()
		var payload bytes.Buffer
		if body != nil {
			if err := json.NewEncoder(&payload).Encode(body); err != nil {
				t.Fatal(err)
			}
		}
		r := httptest.NewRequest(method, path, &payload)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "http://localhost")
		r.RemoteAddr = "192.0.2.100:2345"
		r.Header.Set("X-CSRF-Token", csrf)
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var data map[string]json.RawMessage
		if w.Body.Len() > 0 {
			if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
				t.Fatal(err)
			}
		}
		return w, data
	}
	username := "h_" + strings.ReplaceAll(f.secrets.NewID(), "-", "")[:20]
	w, data := request("POST", "/v1/auth/register", map[string]string{"username": username, "password": "a-secure-long-password"}, nil, "")
	if w.Code != 201 {
		t.Fatalf("register: %d %s", w.Code, w.Body.String())
	}
	if len(data["recovery_codes"]) == 0 || bytes.Contains(w.Body.Bytes(), []byte("password_hash")) {
		t.Fatal("invalid registration response")
	}
	w, data = request("POST", "/v1/auth/login", map[string]string{"username": username, "password": "a-secure-long-password"}, nil, "")
	if w.Code != 200 {
		t.Fatalf("login: %d %s", w.Code, w.Body.String())
	}
	cookie := w.Result().Cookies()[0]
	var csrf string
	_ = json.Unmarshal(data["csrf_token"], &csrf)
	handle := "p_" + strings.ReplaceAll(f.secrets.NewID(), "-", "")[:20]
	w, data = request("POST", "/v1/me/instagram/challenges", map[string]string{"handle": handle}, cookie, csrf)
	if w.Code != 201 {
		t.Fatalf("issue: %d %s", w.Code, w.Body.String())
	}
	var code string
	_ = json.Unmarshal(data["code"], &code)
	var challenge struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(data["challenge"], &challenge)
	w, _ = request("POST", "/v1/me/instagram/challenges/"+challenge.ID+"/sent", nil, cookie, csrf)
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	w, _ = request("POST", "/v1/me/instagram/challenges/"+challenge.ID+"/confirm", nil, cookie, csrf)
	if w.Code != 409 {
		t.Fatal("sent flag unexpectedly confirmed profile")
	}
	otp, err := totp.GenerateCode(secret, f.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	w, data = request("POST", "/v1/auth/login", map[string]string{"username": admin.Username, "password": "admin-password-long", "totp": otp}, nil, "")
	if w.Code != 200 {
		t.Fatalf("admin login: %d %s", w.Code, w.Body.String())
	}
	adminCookie := w.Result().Cookies()[0]
	var adminCSRF string
	_ = json.Unmarshal(data["csrf_token"], &adminCSRF)
	w, _ = request("POST", "/v1/admin/verifications/observe", map[string]string{"code": code, "observed_handle": handle, "reference": "direct in official inbox"}, adminCookie, adminCSRF)
	if w.Code != 200 {
		t.Fatalf("observe: %d %s", w.Code, w.Body.String())
	}
	w, _ = request("POST", "/v1/me/instagram/challenges/"+challenge.ID+"/confirm", nil, cookie, csrf)
	if w.Code != 200 {
		t.Fatalf("confirm: %d %s", w.Code, w.Body.String())
	}
	w, data = request("GET", "/v1/me", nil, cookie, "")
	if w.Code != 200 || !bytes.Contains(data["instagram_profile"], []byte(handle)) {
		t.Fatal(w.Body.String())
	}
	w, _ = request("POST", "/v1/me/instagram/challenges/"+challenge.ID+"/confirm", nil, cookie, csrf)
	if w.Code != 409 {
		t.Fatal("reused challenge")
	}
}
func TestSameProfileConcurrentConfirmationsAreAtomic(t *testing.T) {
	f := setup(t)
	a, b := f.member(t), f.member(t)
	admin, _ := f.admin(t)
	ctx := context.Background()
	handle := "p_" + strings.ReplaceAll(f.secrets.NewID(), "-", "")[:20]
	first, err := f.verification.Issue(ctx, a.User.ID, handle)
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.verification.Issue(ctx, b.User.ID, handle)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []application.IssuedChallenge{first, second} {
		if _, err = f.verification.Observe(ctx, admin, c.Code, handle, "inbox reference"); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, c := range []application.IssuedChallenge{first, second} {
		wg.Go(func() { _, err := f.verification.Confirm(ctx, c.Challenge.UserID, c.Challenge.ID); results <- err })
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, domain.ErrConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success=%d conflict=%d", success, conflict)
	}
	var profiles, verified int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM app.social_profiles WHERE handle=$1 AND revoked_at IS NULL`, handle).Scan(&profiles); err != nil {
		t.Fatal(err)
	}
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM app.verification_challenges WHERE id IN ($1,$2) AND status='verified'`, first.Challenge.ID, second.Challenge.ID).Scan(&verified); err != nil {
		t.Fatal(err)
	}
	if profiles != 1 || verified != 1 {
		t.Fatalf("partial transaction: %d profiles, %d consumed challenges", profiles, verified)
	}
}
func TestRecoveryCodeIsSingleUseAndRevokesSessions(t *testing.T) {
	f := setup(t)
	r := f.member(t)
	ctx := context.Background()
	login, err := f.auth.Login(ctx, r.User.Username, "integration-password-long", "")
	if err != nil {
		t.Fatal(err)
	}
	credential, err := f.store.Credentials(ctx, r.User.Username)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Go(func() { results <- f.auth.Recover(ctx, r.User.Username, r.RecoveryCodes[0], "changed-password-long") })
	}
	wg.Wait()
	close(results)
	success, denied := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, domain.ErrUnauthenticated) {
			denied++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || denied != 1 {
		t.Fatalf("success=%d denied=%d", success, denied)
	}
	if _, err = f.auth.Authenticate(ctx, login.Token); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("old session: %v", err)
	}
	// A login that checked the old password just before recovery cannot create a new session afterward.
	stale := domain.Session{ID: f.secrets.NewID(), UserID: r.User.ID, TokenHash: f.secrets.Digest("stale"), ExpiresAt: f.clock.Now().Add(time.Hour)}
	if err = f.store.CreateSession(ctx, stale, credential.PasswordHash, nil); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("stale login: %v", err)
	}
	if _, err = f.auth.Login(ctx, r.User.Username, "changed-password-long", ""); err != nil {
		t.Fatal(err)
	}
}
func TestNewChallengeCancelsOldAndHidesOtherOwners(t *testing.T) {
	f := setup(t)
	r, other := f.member(t), f.member(t)
	ctx := context.Background()
	a, err := f.verification.Issue(ctx, r.User.ID, "first_profile")
	if err != nil {
		t.Fatal(err)
	}
	b, err := f.verification.Issue(ctx, r.User.ID, "second_profile")
	if err != nil {
		t.Fatal(err)
	}
	old, err := f.verification.Get(ctx, r.User.ID, a.Challenge.ID)
	if err != nil || old.Status != domain.Cancelled {
		t.Fatal(old, err)
	}
	if _, err = f.verification.Get(ctx, other.User.ID, b.Challenge.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal(err)
	}
	if err = f.verification.Sent(ctx, other.User.ID, b.Challenge.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal(err)
	}
	f.clock.now = f.clock.now.Add(24 * time.Hour)
	if _, err = f.verification.Confirm(ctx, r.User.ID, b.Challenge.ID); !errors.Is(err, domain.ErrExpired) {
		t.Fatal(err)
	}
}
func TestLimiterSurvivesNewAdapterAndConcurrentRequests(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	key := f.secrets.NewID()
	var allowed atomic.Int32
	var unexpected atomic.Int32
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			err := f.limiter.Allow(ctx, key, 3, time.Hour, f.clock.Now())
			if err == nil {
				allowed.Add(1)
			} else if !errors.Is(err, domain.ErrLimited) {
				unexpected.Add(1)
			}
		})
	}
	wg.Wait()
	if allowed.Load() != 3 || unexpected.Load() != 0 {
		t.Fatalf("allowed=%d unexpected=%d", allowed.Load(), unexpected.Load())
	}
	if err := postgres.NewLimiter(f.store, f.secrets).Allow(ctx, key, 3, time.Hour, f.clock.Now()); !errors.Is(err, domain.ErrLimited) {
		t.Fatal(err)
	}
}
func TestAdminLoginRequiresTOTPAndRejectsReuse(t *testing.T) {
	f := setup(t)
	admin, secret := f.admin(t)
	ctx := context.Background()
	if _, err := f.auth.Login(ctx, admin.Username, "admin-password-long", ""); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatal(err)
	}
	code, err := totp.GenerateCode(secret, f.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.auth.Login(ctx, admin.Username, "admin-password-long", code); err != nil {
		t.Fatal(err)
	}
	if _, err = f.auth.Login(ctx, admin.Username, "admin-password-long", code); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("reused TOTP: %v", err)
	}
}
