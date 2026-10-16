//go:build e2e

// This fixture is excluded from normal builds and refuses production databases.
package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pquerna/otp/totp"
	"naosemeta.com/backend/internal/adapters/postgres"
	"naosemeta.com/backend/internal/adapters/security"
	"naosemeta.com/backend/internal/domain"
)

func main() {
	if err := run(); err != nil {
		log.Fatal("E2E fixture failed; use a disposable local *_test database in development mode.")
	}
}
func run() error {
	codeOnly := flag.Bool("code", false, "return the current test TOTP code")
	flag.Parse()
	u, err := url.Parse(os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	if os.Getenv("APP_ENV") != "development" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.Hostname() != "::1") || !strings.HasSuffix(u.Path, "_test") {
		return domain.ErrForbidden
	}
	secret := os.Getenv("E2E_TOTP_SECRET")
	if secret == "" {
		return domain.ErrInvalid
	}
	if *codeOnly {
		code, err := totp.GenerateCode(secret, time.Now().UTC())
		if err != nil {
			return err
		}
		fmt.Print(code)
		return nil
	}
	username, err := domain.NormalizeUsername(os.Getenv("E2E_ADMIN_USERNAME"))
	if err != nil {
		return err
	}
	password := os.Getenv("E2E_ADMIN_PASSWORD")
	if err = domain.ValidatePassword(password); err != nil {
		return err
	}
	key, err := base64.StdEncoding.DecodeString(os.Getenv("TOTP_ENCRYPTION_KEY"))
	if err != nil {
		return err
	}
	second, err := security.NewSecondFactor(key)
	if err != nil {
		return err
	}
	passwords, err := security.NewPasswords()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	hash, err := passwords.Hash(ctx, password)
	if err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer pool.Close()
	user := domain.User{ID: security.NewSecrets("e2e").NewID(), Username: username, Role: "admin", CreatedAt: time.Now().UTC()}
	return postgres.New(pool).CreateAccount(ctx, domain.Credential{User: user, PasswordHash: hash, TOTPEncrypted: second.Seal(secret)}, nil)
}
