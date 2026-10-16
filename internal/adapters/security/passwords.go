package security

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Passwords bounds concurrent Argon2 allocations to 128 MiB.
type Passwords struct {
	permits chan struct{}
	dummy   string
}

func NewPasswords() (*Passwords, error) {
	p := &Passwords{permits: make(chan struct{}, 2)}
	var err error
	p.dummy, err = p.Hash(context.Background(), "unavailable-account-placeholder")
	return p, err
}
func (p *Passwords) DummyHash() string { return p.dummy }
func (p *Passwords) acquire(ctx context.Context) error {
	select {
	case p.permits <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (p *Passwords) Hash(ctx context.Context, value string) (string, error) {
	if err := p.acquire(ctx); err != nil {
		return "", err
	}
	defer func() { <-p.permits }()
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(value), salt, 3, 64*1024, 1, 32)
	return "$argon2id$v=19$m=65536,t=3,p=1$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key), nil
}
func (p *Passwords) Verify(ctx context.Context, value, hash string) (bool, error) {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" || parts[3] != "m=65536,t=3,p=1" {
		return false, fmt.Errorf("unsupported password hash")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		return false, fmt.Errorf("invalid password salt")
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) != 32 {
		return false, fmt.Errorf("invalid password digest")
	}
	if err = p.acquire(ctx); err != nil {
		return false, err
	}
	defer func() { <-p.permits }()
	computed := argon2.IDKey([]byte(value), salt, 3, 64*1024, 1, 32)
	return subtle.ConstantTimeCompare(computed, key) == 1, nil
}
