// Package instagram implements Meta's incoming webhook and profile lookup.
package instagram

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"naosemeta.com/backend/internal/domain"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type Receiver interface {
	Receive(context.Context, domain.InstagramMessage) error
}
type Webhook struct {
	secret, verifyToken, accountID string
	receiver                       Receiver
}

var numericID = regexp.MustCompile(`^[0-9]{1,32}$`)

func NewWebhook(secret, verifyToken, accountID string, receiver Receiver) http.Handler {
	return &Webhook{secret, verifyToken, accountID, receiver}
}

func (h *Webhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == http.MethodGet {
		h.subscribe(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		w.WriteHeader(405)
		return
	}
	if h.secret == "" || h.receiver == nil {
		w.WriteHeader(503)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 256<<10))
	if err != nil {
		w.WriteHeader(413)
		return
	}
	signatures := r.Header.Values("X-Hub-Signature-256")
	if len(signatures) != 1 || !strings.HasPrefix(signatures[0], "sha256=") {
		w.WriteHeader(403)
		return
	}
	signature, err := hex.DecodeString(strings.TrimPrefix(signatures[0], "sha256="))
	mac := hmac.New(sha256.New, []byte(h.secret))
	_, _ = mac.Write(body)
	if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		w.WriteHeader(403)
		return
	}
	var payload struct {
		Object string `json:"object"`
		Entry  []struct {
			ID        string `json:"id"`
			Messaging []struct {
				Sender struct {
					ID string `json:"id"`
				} `json:"sender"`
				Recipient struct {
					ID string `json:"id"`
				} `json:"recipient"`
				Timestamp int64 `json:"timestamp"`
				Message   struct {
					ID          string            `json:"mid"`
					Text        string            `json:"text"`
					Echo        bool              `json:"is_echo"`
					Deleted     bool              `json:"is_deleted"`
					Attachments []json.RawMessage `json:"attachments"`
				} `json:"message"`
			} `json:"messaging"`
		} `json:"entry"`
	}
	if json.Unmarshal(body, &payload) != nil {
		w.WriteHeader(400)
		return
	}
	count := 0
	for _, entry := range payload.Entry {
		count += len(entry.Messaging)
	}
	if count > 100 {
		w.WriteHeader(413)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	if payload.Object == "instagram" {
		for _, entry := range payload.Entry {
			if entry.ID != h.accountID {
				continue
			}
			for _, event := range entry.Messaging {
				if event.Recipient.ID != h.accountID || !numericID.MatchString(event.Sender.ID) || event.Sender.ID == h.accountID || event.Message.Echo || event.Message.Deleted || len(event.Message.Attachments) > 0 || event.Message.ID == "" || len(event.Message.ID) > 512 || event.Timestamp <= 0 {
					continue
				}
				message := domain.InstagramMessage{AccountID: h.accountID, SenderID: event.Sender.ID, MessageID: event.Message.ID, Text: event.Message.Text, SentAt: time.UnixMilli(event.Timestamp)}
				if err = h.receiver.Receive(ctx, message); err != nil {
					// Acknowledge only committed work. Redeliveries are idempotent.
					w.Header().Set("Retry-After", "5")
					w.WriteHeader(503)
					return
				}
			}
		}
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, "EVENT_RECEIVED")
}

func (h *Webhook) subscribe(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if len(q["hub.mode"]) != 1 || len(q["hub.verify_token"]) != 1 || len(q["hub.challenge"]) != 1 || q.Get("hub.mode") != "subscribe" || h.verifyToken == "" || q.Get("hub.challenge") == "" || len(q.Get("hub.challenge")) > 512 {
		w.WriteHeader(403)
		return
	}
	want, got := sha256.Sum256([]byte(h.verifyToken)), sha256.Sum256([]byte(q.Get("hub.verify_token")))
	if !hmac.Equal(want[:], got[:]) {
		w.WriteHeader(403)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, q.Get("hub.challenge"))
}
