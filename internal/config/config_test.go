package config_test

import (
	"encoding/base64"
	"strings"
	"testing"

	"naosemeta.com/backend/internal/config"
)

func TestProductionFailsClosed(t *testing.T) {
	key := make([]byte, 32)
	key[0] = 1
	base := map[string]string{"APP_ENV": "production", "PUBLIC_BASE_URL": "https://example.test", "DATABASE_URL": "postgres://app:password@db.example.test/app?sslmode=verify-full", "INSTAGRAM_OFFICIAL_HANDLE": "projeto", "VERIFICATION_CODE_PEPPER": "random-pepper-with-at-least-32-bytes", "TOTP_ENCRYPTION_KEY": base64.StdEncoding.EncodeToString(key)}
	if _, err := config.Parse(func(k string) string { return base[k] }); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{"DATABASE_URL": "postgres://app:password@db.example.test/app?sslmode=disable", "PUBLIC_BASE_URL": "http://example.test", "DATABASE_MAX_CONNECTIONS": "100", "VERIFICATION_CODE_PEPPER": "short", "TOTP_ENCRYPTION_KEY": "invalid", "TRUSTED_PROXY_CIDRS": "0.0.0.0/0", "INSTAGRAM_OFFICIAL_HANDLE": "https://instagram.com/a"} {
		t.Run(key, func(t *testing.T) {
			if _, err := config.Parse(func(k string) string {
				if k == key {
					return value
				}
				return base[k]
			}); err == nil {
				t.Fatal("accepted unsafe configuration")
			}
		})
	}
}

func TestWebhookConfigurationCannotBePartiallyEnabled(t *testing.T) {
	key := make([]byte, 32)
	key[0] = 1
	values := map[string]string{"APP_ENV": "production", "PUBLIC_BASE_URL": "https://example.test", "DATABASE_URL": "postgres://app:password@db.example.test/app?sslmode=verify-full", "INSTAGRAM_OFFICIAL_HANDLE": "projeto", "VERIFICATION_CODE_PEPPER": strings.Repeat("p", 64), "TOTP_ENCRYPTION_KEY": base64.StdEncoding.EncodeToString(key)}
	get := func(k string) string { return values[k] }
	c, err := config.Parse(get)
	if err != nil || c.InstagramMode != "manual" {
		t.Fatal("unsafe default", err)
	}
	values["INSTAGRAM_VERIFICATION_MODE"] = "webhook"
	if _, err = config.Parse(get); err == nil {
		t.Fatal("accepted missing credentials")
	}
	for k, v := range map[string]string{"META_APP_SECRET": strings.Repeat("s", 32), "META_WEBHOOK_VERIFY_TOKEN": strings.Repeat("v", 32), "INSTAGRAM_ACCESS_TOKEN": strings.Repeat("t", 64), "INSTAGRAM_ACCOUNT_ID": "123456", "META_GRAPH_API_VERSION": "v25.0"} {
		values[k] = v
	}
	if _, err = config.Parse(get); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{"INSTAGRAM_VERIFICATION_MODE": "automatic", "META_APP_SECRET": "", "META_WEBHOOK_VERIFY_TOKEN": "short", "INSTAGRAM_ACCESS_TOKEN": "token\ninjection", "INSTAGRAM_ACCOUNT_ID": "../me", "META_GRAPH_API_VERSION": "https://evil.example/"} {
		t.Run(k, func(t *testing.T) {
			old := values[k]
			values[k] = v
			defer func() { values[k] = old }()
			if _, err = config.Parse(get); err == nil {
				t.Fatal("accepted unsafe webhook config")
			}
		})
	}
}

func TestDevelopmentKeysCannotBeUsedInProduction(t *testing.T) {
	values := map[string]string{"APP_ENV": "production", "PUBLIC_BASE_URL": "https://example.test", "DATABASE_URL": "postgres://app:password@db.example.test/app?sslmode=verify-full", "INSTAGRAM_OFFICIAL_HANDLE": "projeto", "VERIFICATION_CODE_PEPPER": "local-development-only-not-for-production-123456", "TOTP_ENCRYPTION_KEY": base64.StdEncoding.EncodeToString(make([]byte, 32))}
	if _, err := config.Parse(func(k string) string { return values[k] }); err == nil {
		t.Fatal("accepted development keys")
	}
	values["VERIFICATION_CODE_PEPPER"] = "independent-random-production-pepper-123456"
	if _, err := config.Parse(func(k string) string { return values[k] }); err == nil {
		t.Fatal("accepted zero TOTP key")
	}
}
