package config_test

import (
	"encoding/base64"
	"encoding/hex"
	"naosemeta.com/backend/internal/config"
	"strings"
	"testing"
)

func TestProductionDatabaseCannotOverrideTLSOrDestination(t *testing.T) {
	for _, suffix := range []string{
		"sslmode=verify-full&sslmode=disable",
		"sslmode=verify-full&ssl=true",
		"sslmode=verify-full&host=%2Ftmp",
		"sslmode=verify-full&service=another",
		"sslmode=verify-full&password=another",
	} {
		if err := config.ValidateDatabaseURL("postgres://app:fake-test-password@db.example.test/app?"+suffix, "production"); err == nil {
			t.Errorf("accepted connection override: %s", suffix)
		}
	}
}

func TestLocalListenerAndIndependentKeyMaterial(t *testing.T) {
	key := make([]byte, 32)
	key[0] = 42
	values := map[string]string{"APP_ENV": "development", "PUBLIC_BASE_URL": "http://127.0.0.1:8080", "DATABASE_URL": "postgres://app:fake-test-password@localhost/app?sslmode=disable", "INSTAGRAM_OFFICIAL_HANDLE": "projeto", "VERIFICATION_CODE_PEPPER": strings.Repeat("p", 64), "TOTP_ENCRYPTION_KEY": base64.StdEncoding.EncodeToString(key)}
	get := func(k string) string { return values[k] }
	c, err := config.Parse(get)
	if err != nil || c.ListenHost != "127.0.0.1" {
		t.Fatal("development listener must be loopback", err)
	}
	values["APP_ENV"] = "production"
	values["PUBLIC_BASE_URL"] = "https://example.test"
	values["DATABASE_URL"] = "postgres://app:fake-test-password@db.example.test/app?sslmode=verify-full"
	c, err = config.Parse(get)
	if err != nil || c.ListenHost != "0.0.0.0" {
		t.Fatal("container listener invalid", err)
	}
	for _, pepper := range []string{string(key), hex.EncodeToString(key), base64.StdEncoding.EncodeToString(key)} {
		values["VERIFICATION_CODE_PEPPER"] = pepper
		if _, err = config.Parse(get); err == nil {
			t.Fatal("same pepper and TOTP key accepted")
		}
	}
	values["VERIFICATION_CODE_PEPPER"] = strings.Repeat("p", 64)
	values["INSTAGRAM_VERIFICATION_MODE"] = "webhook"
	values["META_APP_SECRET"] = strings.Repeat("s", 32)
	values["META_WEBHOOK_VERIFY_TOKEN"] = values["META_APP_SECRET"]
	values["INSTAGRAM_ACCESS_TOKEN"] = strings.Repeat("t", 64)
	values["INSTAGRAM_ACCOUNT_ID"] = "123"
	values["META_GRAPH_API_VERSION"] = "v25.0"
	if _, err = config.Parse(get); err == nil {
		t.Fatal("verify token reused as app secret")
	}
	values["META_WEBHOOK_VERIFY_TOKEN"] = base64.StdEncoding.EncodeToString([]byte(values["META_APP_SECRET"]))
	if _, err = config.Parse(get); err == nil {
		t.Fatal("encoded app secret reused as verify token")
	}
	values["META_WEBHOOK_VERIFY_TOKEN"] = strings.Repeat("v", 32)
	values["INSTAGRAM_ACCESS_TOKEN"] = "synthetic-e2e-token-never-use-in-production"
	if _, err = config.Parse(get); err == nil {
		t.Fatal("E2E token accepted in production")
	}
}
