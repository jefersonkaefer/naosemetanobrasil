package domain

import (
	"regexp"
	"strings"
	"time"
)

var localUsername = regexp.MustCompile(`^[a-z0-9_]{3,32}$`)
var instagramHandle = regexp.MustCompile(`^[a-z0-9_](?:[a-z0-9_.]{0,28}[a-z0-9_])?$`)

func NormalizeUsername(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if !localUsername.MatchString(value) {
		return "", ErrInvalid
	}
	return value, nil
}

func NormalizeHandle(value string) (string, error) {
	value = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(value), "@"))
	if !instagramHandle.MatchString(value) || strings.Contains(value, "..") {
		return "", ErrInvalid
	}
	return value, nil
}

func ValidatePassword(value string) error {
	if len(value) < 12 || len(value) > 128 {
		return ErrInvalid
	}
	return nil
}

type User struct {
	ID          string
	Username    string
	DisplayName string
	Role        string
	CreatedAt   time.Time
}

func (u User) IsAdmin() bool { return u.Role == "admin" }

type Credential struct {
	User          User
	PasswordHash  string
	TOTPEncrypted []byte
}

type Session struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
}

type SocialProfile struct {
	ID         string
	UserID     string
	Platform   string
	Handle     string
	Method     string
	VerifiedAt time.Time
}
