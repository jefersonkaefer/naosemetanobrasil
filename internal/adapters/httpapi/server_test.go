package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"naosemeta.com/backend/internal/adapters/security"
	"naosemeta.com/backend/internal/application"
	"naosemeta.com/backend/internal/domain"
)

type fakeAuth struct {
	AuthUseCases
	user domain.User
}

func (f fakeAuth) Authenticate(context.Context, string) (domain.User, error) { return f.user, nil }
func (fakeAuth) CSRF(string) string                                          { return "expected-csrf" }
func (f fakeAuth) Login(context.Context, string, string, string) (application.Login, error) {
	return application.Login{User: f.user, Token: strings.Repeat("a", 43), CSRFToken: "expected-csrf", ExpiresAt: time.Now().Add(time.Hour)}, nil
}

type fakeVerification struct {
	VerificationUseCases
	called *bool
}

func (f fakeVerification) Issue(context.Context, string, string) (application.IssuedChallenge, error) {
	*f.called = true
	return application.IssuedChallenge{OfficialHandle: "projeto"}, nil
}
func (fakeVerification) Get(context.Context, string, string) (domain.Challenge, error) {
	return domain.Challenge{ID: "returned", CodeHash: "secret-code-hash", ReviewReference: "private-reference", ReviewedBy: "admin-id", ObservedAccountID: "private-account-id", ObservedSenderID: "private-sender-id"}, nil
}

type allowAll struct{}

func (allowAll) Allow(context.Context, string, int, time.Duration, time.Time) error { return nil }

func fixture(t *testing.T) (http.Handler, *bool) {
	t.Helper()
	called := new(bool)
	h := New(fakeAuth{user: domain.User{ID: "owner", Role: "member"}}, fakeVerification{called: called}, allowAll{}, security.Clock{}, security.NewSecrets("test"), Options{Origin: "https://platform.test", SecureCookies: true, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Ready: func(context.Context) error { return nil }})
	return h, called
}
func TestCSRFAndOriginAreRequiredForMutation(t *testing.T) {
	for _, tc := range []struct {
		name, csrf, origin string
		status             int
	}{{"missing csrf", "", "https://platform.test", 403}, {"wrong origin", "expected-csrf", "https://evil.test", 403}, {"valid", "expected-csrf", "https://platform.test", 201}} {
		t.Run(tc.name, func(t *testing.T) {
			h, called := fixture(t)
			req := httptest.NewRequest("POST", "/v1/me/instagram/challenges", strings.NewReader(`{"handle":"pessoa"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("X-CSRF-Token", tc.csrf)
			req.AddCookie(&http.Cookie{Name: secureCookieName, Value: "session"})
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("%d: %s", w.Code, w.Body.String())
			}
			if *called != (tc.status == 201) {
				t.Fatal("unexpected mutation")
			}
		})
	}
}
func TestRejectUnknownFieldsTrailingJSONAndLargePayload(t *testing.T) {
	for _, body := range []string{`{"handle":"pessoa","role":"admin"}`, `{"handle":"pessoa"} {}`, `{"handle":"` + strings.Repeat("x", 65536) + `"}`} {
		h, called := fixture(t)
		req := httptest.NewRequest("POST", "/v1/me/instagram/challenges", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", "expected-csrf")
		req.AddCookie(&http.Cookie{Name: secureCookieName, Value: "session"})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 400 && w.Code != 413 {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
		if *called {
			t.Fatal("mutated with invalid JSON")
		}
	}
}
func TestNonAdminCannotAccessQueue(t *testing.T) {
	h, _ := fixture(t)
	req := httptest.NewRequest("GET", "/v1/admin/verifications", nil)
	req.AddCookie(&http.Cookie{Name: secureCookieName, Value: "session"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
}
func TestSessionCookieIsSecureAndOpaque(t *testing.T) {
	h, _ := fixture(t)
	req := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader(`{"username":"pessoa","password":"long-enough-password"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != secureCookieName || cookies[0].Domain != "" || cookies[0].Path != "/" || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatal("insecure cookie")
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["token"]; ok {
		t.Fatal("exposed session token in JSON")
	}
}
func TestResponsesDoNotLeakInternalVerificationFields(t *testing.T) {
	h, _ := fixture(t)
	req := httptest.NewRequest("GET", "/v1/me/instagram/challenges/00000000-0000-4000-8000-000000000001", nil)
	req.AddCookie(&http.Cookie{Name: secureCookieName, Value: "session"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	for _, secret := range []string{"secret-code-hash", "private-reference", "admin-id", "private-account-id", "private-sender-id"} {
		if strings.Contains(w.Body.String(), secret) {
			t.Errorf("leaked %s", secret)
		}
	}
}

func TestProductionRejectsLegacyCookieAndSetsHeadersOnErrors(t *testing.T) {
	h, _ := fixture(t)
	r := httptest.NewRequest("GET", "/v1/me", nil)
	r.AddCookie(&http.Cookie{Name: cookieName, Value: "legacy-session"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("production accepted unprefixed cookie")
	}
	for key, value := range map[string]string{"Strict-Transport-Security": "max-age=31536000", "X-Frame-Options": "DENY", "Referrer-Policy": "no-referrer", "Cache-Control": "no-store"} {
		if w.Header().Get(key) != value {
			t.Errorf("missing %s", key)
		}
	}
	if w.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("API errors lack CSP")
	}
}
func TestForwardedIPRequiresTrustedProxyAndIgnoresSpoofedPrefix(t *testing.T) {
	s := Server{options: Options{TrustedProxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}}
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.0.2.1:80"
	r.Header.Set("X-Forwarded-For", "1.1.1.1")
	if ip := s.clientIP(r); ip != "192.0.2.1" {
		t.Fatal(ip)
	}
	r.RemoteAddr = "10.0.0.1:80"
	r.Header.Set("X-Forwarded-For", "1.1.1.1, 192.0.2.2, 10.0.0.2")
	if ip := s.clientIP(r); ip != "192.0.2.2" {
		t.Fatal(ip)
	}
}
