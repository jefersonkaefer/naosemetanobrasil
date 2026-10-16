package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"regexp"
	"strings"
	"time"

	"naosemeta.com/backend/internal/application"
	"naosemeta.com/backend/internal/domain"
)

type Options struct {
	Origin           string
	SecureCookies    bool
	TrustedProxies   []netip.Prefix
	Logger           *slog.Logger
	Ready            func(context.Context) error
	Cleanup          func(context.Context) error
	Frontend         http.Handler
	InstagramWebhook http.Handler
	InstagramMode    string
}
type Server struct {
	auth         AuthUseCases
	verification VerificationUseCases
	limiter      application.RateLimiter
	clock        application.Clock
	secrets      application.Secrets
	options      Options
}
type principal struct {
	user  domain.User
	token string
}
type contextKey int

const principalKey contextKey = 0
const requestIDKey contextKey = 1
const cookieName = "nsm_session"
const secureCookieName = "__Host-nsm_session"

func (s *Server) sessionCookieName() string {
	if s.options.SecureCookies {
		return secureCookieName
	}
	return cookieName
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func New(auth AuthUseCases, verification VerificationUseCases, limiter application.RateLimiter, clock application.Clock, secrets application.Secrets, options Options) http.Handler {
	s := &Server{auth, verification, limiter, clock, secrets, options}
	mux := http.NewServeMux()
	if options.InstagramWebhook != nil {
		mux.Handle("/v1/webhooks/instagram", options.InstagramWebhook)
	}
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) { s.json(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := s.options.Ready(ctx); err != nil {
			s.failure(w, r, err)
			return
		}
		s.json(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /v1/auth/register", s.register)
	mux.HandleFunc("POST /v1/auth/login", s.login)
	mux.HandleFunc("POST /v1/auth/recover", s.recoverAccount)
	mux.Handle("POST /v1/auth/logout", s.authenticated(http.HandlerFunc(s.logout)))
	mux.Handle("GET /v1/me", s.authenticated(http.HandlerFunc(s.me)))
	mux.Handle("POST /v1/me/instagram/challenges", s.authenticated(http.HandlerFunc(s.issue)))
	mux.Handle("GET /v1/me/instagram/challenges/{id}", s.authenticated(http.HandlerFunc(s.challenge)))
	mux.Handle("POST /v1/me/instagram/challenges/{id}/sent", s.authenticated(http.HandlerFunc(s.sent)))
	mux.Handle("POST /v1/me/instagram/challenges/{id}/confirm", s.authenticated(http.HandlerFunc(s.confirm)))
	mux.Handle("DELETE /v1/me/social-profiles/{id}", s.authenticated(http.HandlerFunc(s.revoke)))
	mux.Handle("GET /v1/admin/verifications", s.admin(http.HandlerFunc(s.pending)))
	mux.Handle("POST /v1/admin/verifications/observe", s.admin(http.HandlerFunc(s.observe)))
	mux.Handle("POST /v1/admin/verifications/{id}/reject", s.admin(http.HandlerFunc(s.reject)))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if s.options.Frontend != nil && (r.Method == http.MethodGet || r.Method == http.MethodHead) && r.URL.Path != "/v1" && !strings.HasPrefix(r.URL.Path, "/v1/") && !strings.HasPrefix(r.URL.Path, "/health") {
			s.options.Frontend.ServeHTTP(w, r)
			return
		}
		s.failure(w, r, domain.ErrNotFound)
	})
	return s.middleware(mux)
}

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		id := s.secrets.NewID()
		r = r.WithContext(context.WithValue(r.Context(), requestIDKey, id))
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		if s.options.SecureCookies {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		defer func() {
			if recovered := recover(); recovered != nil {
				s.options.Logger.Error("request panic", "request_id", id)
				s.failure(w, r, errors.New("request panic"))
			}
		}()
		if origin := r.Header.Get("Origin"); origin != "" && origin != s.options.Origin {
			s.failure(w, r, domain.ErrForbidden)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/") {
			if !s.allow(w, r, "http:"+s.clientIP(r), 120, time.Minute) {
				return
			}
			// Opportunistic, bounded maintenance only on real traffic.
			if s.options.Cleanup != nil && id[len(id)-1] == '0' {
				ctx, cancel := context.WithTimeout(r.Context(), time.Second)
				if err := s.options.Cleanup(ctx); err != nil {
					s.options.Logger.Warn("maintenance deferred", "request_id", id)
				}
				cancel()
			}
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) authenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(s.sessionCookieName())
		if err != nil {
			s.failure(w, r, domain.ErrUnauthenticated)
			return
		}
		u, err := s.auth.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			s.failure(w, r, err)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			expected := s.auth.CSRF(cookie.Value)
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(expected)) != 1 {
				s.failure(w, r, domain.ErrForbidden)
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey, principal{u, cookie.Value})))
	})
}
func (s *Server) admin(next http.Handler) http.Handler {
	return s.authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !who(r).user.IsAdmin() {
			s.failure(w, r, domain.ErrForbidden)
			return
		}
		next.ServeHTTP(w, r)
	}))
}
func who(r *http.Request) principal { return r.Context().Value(principalKey).(principal) }
func (s *Server) allow(w http.ResponseWriter, r *http.Request, key string, limit int, window time.Duration) bool {
	if err := s.limiter.Allow(r.Context(), key, limit, window, s.clock.Now()); err != nil {
		if errors.Is(err, domain.ErrLimited) {
			w.Header().Set("Retry-After", "60")
		}
		s.failure(w, r, err)
		return false
	}
	return true
}
func (s *Server) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return "unknown"
	}
	trusted := func(ip netip.Addr) bool {
		for _, prefix := range s.options.TrustedProxies {
			if prefix.Contains(ip) {
				return true
			}
		}
		return false
	}
	if !trusted(addr) {
		return addr.String()
	}
	parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(parts) - 1; i >= 0; i-- {
		candidate, err := netip.ParseAddr(strings.TrimSpace(parts[i]))
		if err != nil {
			return addr.String()
		}
		addr = candidate
		if !trusted(addr) {
			return addr.String()
		}
	}
	return addr.String()
}
func (s *Server) decode(w http.ResponseWriter, r *http.Request, value any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		s.failure(w, r, domain.ErrInvalid)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(value); err != nil {
		s.decodeFailure(w, r, err)
		return false
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			err = domain.ErrInvalid
		}
		s.decodeFailure(w, r, err)
		return false
	}
	return true
}
func (s *Server) decodeFailure(w http.ResponseWriter, r *http.Request, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		s.json(w, 413, map[string]any{"error": map[string]any{"code": "payload_too_large", "message": "Pedido muito grande.", "request_id": r.Context().Value(requestIDKey)}})
		return
	}
	s.failure(w, r, domain.ErrInvalid)
}
func (s *Server) pathID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		s.failure(w, r, domain.ErrInvalid)
		return "", false
	}
	return id, true
}
func (s *Server) json(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func (s *Server) failure(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := 503, "unavailable", "Serviço temporariamente indisponível."
	switch {
	case errors.Is(err, domain.ErrInvalid):
		status, code, message = 400, "invalid_input", "Confira os dados enviados."
	case errors.Is(err, domain.ErrUnauthenticated):
		status, code, message = 401, "unauthenticated", "Não foi possível autenticar."
	case errors.Is(err, domain.ErrForbidden):
		status, code, message = 403, "forbidden", "Operação não permitida."
	case errors.Is(err, domain.ErrNotFound):
		status, code, message = 404, "not_found", "Registro não encontrado."
	case errors.Is(err, domain.ErrConflict):
		status, code, message = 409, "conflict", "O perfil ou registro já está em uso, ou não corresponde ao esperado."
	case errors.Is(err, domain.ErrExpired):
		status, code, message = 409, "challenge_expired", "O código expirou. Solicite um novo."
	case errors.Is(err, domain.ErrTransition):
		status, code, message = 409, "invalid_transition", "Esta operação não está disponível no estado atual."
	case errors.Is(err, domain.ErrLimited):
		status, code, message = 429, "rate_limited", "Aguarde antes de tentar novamente."
	default:
		s.options.Logger.Error("request failed", "request_id", r.Context().Value(requestIDKey))
	}
	s.json(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "request_id": r.Context().Value(requestIDKey)}})
}
