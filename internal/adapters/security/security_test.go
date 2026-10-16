package security_test

import (
	"context"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"naosemeta.com/backend/internal/adapters/security"
)

func TestPasswordsAreSaltedAndRejectWrongPassword(t *testing.T) {
	p, err := security.NewPasswords()
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Hash(context.Background(), "a sufficiently long password")
	if err != nil {
		t.Fatal(err)
	}
	b, err := p.Hash(context.Background(), "a sufficiently long password")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("hashes have no unique salt")
	}
	for _, input := range []struct {
		password string
		want     bool
	}{{"a sufficiently long password", true}, {"wrong password", false}} {
		got, err := p.Verify(context.Background(), input.password, a)
		if err != nil || got != input.want {
			t.Fatalf("verify: %v %v", got, err)
		}
	}
	if _, err := p.Verify(context.Background(), "x", "$argon2id$v=19$m=999999999,t=3,p=1$YQ$YQ"); err == nil {
		t.Fatal("accepted unbounded parameters")
	}
}
func TestTOTPEncryptionAndCounter(t *testing.T) {
	second, err := security.NewSecondFactor(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	secret := "JBSWY3DPEHPK3PXP"
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	code, err := totp.GenerateCode(secret, now)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext := second.Seal(secret)
	counter, err := second.Verify(ciphertext, code, now)
	if err != nil || counter != now.Unix()/30 {
		t.Fatalf("%d %v", counter, err)
	}
	ciphertext[len(ciphertext)-1] ^= 1
	if _, err = second.Verify(ciphertext, code, now); err == nil {
		t.Fatal("accepted altered ciphertext")
	}
	if _, err = second.Verify(second.Seal(secret), code, now.Add(2*time.Minute)); err == nil {
		t.Fatal("accepted old TOTP")
	}
}
