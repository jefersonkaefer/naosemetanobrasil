package instagram

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"naosemeta.com/backend/internal/domain"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type receiverFunc func(context.Context, domain.InstagramMessage) error

func (f receiverFunc) Receive(ctx context.Context, m domain.InstagramMessage) error { return f(ctx, m) }
func signedRequest(body, secret string) *http.Request {
	r := httptest.NewRequest("POST", "/v1/webhooks/instagram", strings.NewReader(body))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	r.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	return r
}
func TestWebhookSignatureAndFiltering(t *testing.T) {
	base := `{"object":"instagram","entry":[{"id":"123","messaging":[{"sender":{"id":"456"},"recipient":{"id":"123"},"timestamp":1700000000123,"message":{"mid":"message-1","text":"NSM-a_code","is_echo":false}}]}]}`
	for _, tc := range []struct {
		name, body, secret string
		status, calls      int
	}{
		{"valid", base, "secret", 200, 1},
		{"bad signature", base, "other", 403, 0},
		{"echo", strings.Replace(base, `"is_echo":false`, `"is_echo":true`, 1), "secret", 200, 0},
		{"deleted", strings.Replace(base, `"is_echo":false`, `"is_deleted":true`, 1), "secret", 200, 0},
		{"attachment", strings.Replace(base, `"is_echo":false`, `"attachments":[{"type":"image"}]`, 1), "secret", 200, 0},
		{"other object", strings.Replace(base, `"instagram"`, `"page"`, 1), "secret", 200, 0},
		{"other account", strings.Replace(base, `"id":"123"`, `"id":"999"`, 1), "secret", 200, 0},
		{"other recipient", strings.Replace(base, `"recipient":{"id":"123"}`, `"recipient":{"id":"999"}`, 1), "secret", 200, 0},
		{"unsafe sender", strings.Replace(base, `"id":"456"`, `"id":"../me"`, 1), "secret", 200, 0},
		{"malformed", "{", "secret", 400, 0},
		{"oversized", strings.Repeat("x", (256<<10)+1), "secret", 413, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			h := NewWebhook("secret", "verify", "123", receiverFunc(func(_ context.Context, m domain.InstagramMessage) error {
				calls++
				if m.AccountID != "123" || m.SenderID != "456" || m.MessageID != "message-1" || m.SentAt.UnixMilli() != 1700000000123 {
					t.Fatal("wrong identity evidence")
				}
				return nil
			}))
			w := httptest.NewRecorder()
			h.ServeHTTP(w, signedRequest(tc.body, tc.secret))
			if w.Code != tc.status || calls != tc.calls {
				t.Fatalf("status=%d calls=%d", w.Code, calls)
			}
		})
	}
}
func TestWebhookSubscriptionAndRetry(t *testing.T) {
	h := NewWebhook("secret", "verify", "123", receiverFunc(func(context.Context, domain.InstagramMessage) error { return errors.New("temporary failure") }))
	for _, tc := range []struct {
		query  string
		status int
	}{
		{"hub.mode=subscribe&hub.verify_token=verify&hub.challenge=challenge", 200},
		{"hub.mode=subscribe&hub.verify_token=wrong&hub.challenge=challenge", 403},
		{"hub.mode=subscribe&hub.verify_token=verify&hub.verify_token=verify&hub.challenge=challenge", 403},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/v1/webhooks/instagram?"+tc.query, nil))
		if w.Code != tc.status {
			t.Fatal(w.Code)
		}
		if tc.status == 200 && w.Body.String() != "challenge" {
			t.Fatal("challenge mismatch")
		}
	}
	body := `{"object":"instagram","entry":[{"id":"123","messaging":[{"sender":{"id":"456"},"recipient":{"id":"123"},"timestamp":1700000000000,"message":{"mid":"retry","text":"code"}}]}]}`
	w := httptest.NewRecorder()
	h.ServeHTTP(w, signedRequest(body, "secret"))
	if w.Code != 503 || w.Header().Get("Retry-After") == "" {
		t.Fatal("failure acknowledged as success")
	}
	for _, signature := range []string{"", "sha256=wrong", "sha1=abc"} {
		r := signedRequest(body, "secret")
		r.Header.Set("X-Hub-Signature-256", signature)
		w = httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("invalid signature accepted")
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestProfileLookupAuthenticatesIdentityAndKeepsTokenPrivate(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		ok         bool
	}{
		{"valid", `{"id":"456","username":"Pessoa"}`, 200, true},
		{"wrong identity", `{"id":"999","username":"pessoa"}`, 200, false},
		{"invalid handle", `{"id":"456","username":"https://example.com"}`, 200, false},
		{"upstream failed", `{"error":"sensitive upstream detail"}`, 500, false},
		{"oversized", strings.Repeat("x", (64<<10)+1), 200, false},
		{"redirect", "", 302, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := NewProfiles("private-token", "v25.0")
			calls := 0
			p.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.String() != "https://graph.instagram.com/v25.0/456?fields=id,username" || r.Header.Get("Authorization") != "Bearer private-token" {
					t.Fatal("wrong target or token transport")
				}
				return &http.Response{StatusCode: tc.status, Header: http.Header{"Location": []string{"https://example.invalid/"}}, Body: io.NopCloser(strings.NewReader(tc.body)), Request: r}, nil
			})
			handle, err := p.Handle(context.Background(), "456")
			if tc.ok {
				if err != nil || handle != "pessoa" {
					t.Fatal(handle, err)
				}
			} else if err == nil || strings.Contains(fmt.Sprint(err), "private-token") || strings.Contains(fmt.Sprint(err), "sensitive") {
				t.Fatal("unsafe profile failure", err)
			}
			if calls != 1 {
				t.Fatal("unexpected retry or redirect")
			}
			if _, err = p.Handle(context.Background(), "../me"); err == nil || calls != 1 {
				t.Fatal("path injection accepted")
			}
		})
	}
}
