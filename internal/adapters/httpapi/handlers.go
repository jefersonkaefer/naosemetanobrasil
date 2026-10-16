package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"naosemeta.com/backend/internal/domain"
)

func userView(u domain.User) map[string]any {
	return map[string]any{"id": u.ID, "username": u.Username, "display_name": u.DisplayName, "role": u.Role, "created_at": u.CreatedAt}
}
func challengeView(c domain.Challenge) map[string]any {
	return map[string]any{"id": c.ID, "expected_handle": c.ExpectedHandle, "observed_handle": c.ObservedHandle, "status": c.Status, "created_at": c.CreatedAt, "expires_at": c.ExpiresAt, "sent_at": c.SentAt, "verification_method": c.Method}
}
func profileView(p domain.SocialProfile) map[string]any {
	return map[string]any{"id": p.ID, "platform": p.Platform, "handle": p.Handle, "verification_method": p.Method, "verified_at": p.VerifiedAt, "public_visibility": false}
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	if !s.allow(w, r, "register:"+s.clientIP(r), 5, time.Hour) {
		return
	}
	var input struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	result, err := s.auth.Register(r.Context(), input.Username, input.Password, input.DisplayName)
	if err != nil {
		s.failure(w, r, err)
		return
	}
	s.json(w, 201, map[string]any{"user": userView(result.User), "recovery_codes": result.RecoveryCodes})
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !s.allow(w, r, "login-ip:"+s.clientIP(r), 20, time.Minute) {
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
		TOTP     string `json:"totp"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	if !s.allow(w, r, "login-user:"+strings.ToLower(strings.TrimSpace(input.Username)), 10, time.Minute) {
		return
	}
	result, err := s.auth.Login(r.Context(), input.Username, input.Password, input.TOTP)
	if err != nil {
		s.failure(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: s.sessionCookieName(), Value: result.Token, Path: "/", HttpOnly: true, Secure: s.options.SecureCookies, SameSite: http.SameSiteLaxMode, Expires: result.ExpiresAt, MaxAge: 7 * 24 * 60 * 60})
	s.json(w, 200, map[string]any{"user": userView(result.User), "csrf_token": result.CSRFToken, "expires_at": result.ExpiresAt})
}
func (s *Server) recoverAccount(w http.ResponseWriter, r *http.Request) {
	if !s.allow(w, r, "recover:"+s.clientIP(r), 5, time.Hour) {
		return
	}
	var input struct {
		Username string `json:"username"`
		Code     string `json:"recovery_code"`
		Password string `json:"new_password"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	if !s.allow(w, r, "recover-user:"+strings.ToLower(strings.TrimSpace(input.Username)), 5, time.Hour) {
		return
	}
	if err := s.auth.Recover(r.Context(), input.Username, input.Code, input.Password); err != nil {
		s.failure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if err := s.auth.Logout(r.Context(), who(r).token); err != nil {
		s.failure(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: s.sessionCookieName(), Value: "", Path: "/", HttpOnly: true, Secure: s.options.SecureCookies, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	p, err := s.verification.Profile(r.Context(), who(r).user.ID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		s.failure(w, r, err)
		return
	}
	var profile any
	if err == nil {
		profile = profileView(p)
	}
	s.json(w, 200, map[string]any{"user": userView(who(r).user), "instagram_profile": profile, "csrf_token": s.auth.CSRF(who(r).token)})
}
func (s *Server) issue(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Handle string `json:"handle"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	result, err := s.verification.Issue(r.Context(), who(r).user.ID, input.Handle)
	if err != nil {
		s.failure(w, r, err)
		return
	}
	mode := s.options.InstagramMode
	if mode == "" {
		mode = "manual"
	}
	view := challengeView(result.Challenge)
	view["verification_mode"] = mode
	s.json(w, 201, map[string]any{"challenge": view, "code": result.Code, "official_handle": result.OfficialHandle, "direct_url": "https://ig.me/m/" + result.OfficialHandle, "verification_mode": mode})
}
func (s *Server) challenge(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r)
	if !ok {
		return
	}
	c, err := s.verification.Get(r.Context(), who(r).user.ID, id)
	if err != nil {
		s.failure(w, r, err)
		return
	}
	view := challengeView(c)
	mode := s.options.InstagramMode
	if mode == "" {
		mode = "manual"
	}
	view["verification_mode"] = mode
	s.json(w, 200, view)
}
func (s *Server) sent(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r)
	if !ok {
		return
	}
	if err := s.verification.Sent(r.Context(), who(r).user.ID, id); err != nil {
		s.failure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) confirm(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r)
	if !ok {
		return
	}
	p, err := s.verification.Confirm(r.Context(), who(r).user.ID, id)
	if err != nil {
		s.failure(w, r, err)
		return
	}
	s.json(w, 200, profileView(p))
}
func (s *Server) revoke(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r)
	if !ok {
		return
	}
	if err := s.verification.Revoke(r.Context(), who(r).user.ID, id); err != nil {
		s.failure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) pending(w http.ResponseWriter, r *http.Request) {
	challenges, err := s.verification.Pending(r.Context(), who(r).user)
	if err != nil {
		s.failure(w, r, err)
		return
	}
	items := make([]map[string]any, 0, len(challenges))
	for _, c := range challenges {
		item := challengeView(c)
		item["user_id"] = c.UserID
		items = append(items, item)
	}
	s.json(w, 200, map[string]any{"items": items, "limit": 50})
}
func (s *Server) observe(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Code      string `json:"code"`
		Handle    string `json:"observed_handle"`
		Reference string `json:"reference"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	c, err := s.verification.Observe(r.Context(), who(r).user, input.Code, input.Handle, strings.TrimSpace(input.Reference))
	if err != nil {
		s.failure(w, r, err)
		return
	}
	s.json(w, 200, challengeView(c))
}
func (s *Server) reject(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r)
	if !ok {
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	if err := s.verification.Reject(r.Context(), who(r).user, id, strings.TrimSpace(input.Reason)); err != nil {
		s.failure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
