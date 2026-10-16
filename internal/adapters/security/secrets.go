package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/pquerna/otp/totp"
	"naosemeta.com/backend/internal/domain"
)

type Clock struct{}

func (Clock) Now() time.Time { return time.Now().UTC() }

type Secrets struct{ pepper []byte }

func NewSecrets(pepper string) *Secrets { return &Secrets{[]byte(pepper)} }
func (*Secrets) NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
func (*Secrets) Token(size int) string {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func (s *Secrets) Digest(value string) string {
	mac := hmac.New(sha256.New, s.pepper)
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

type SecondFactor struct{ aead cipher.AEAD }

func NewSecondFactor(key []byte) (*SecondFactor, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &SecondFactor{aead}, nil
}
func (s *SecondFactor) Seal(secret string) []byte {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic(err)
	}
	return s.aead.Seal(nonce, nonce, []byte(secret), []byte("nsm:admin-totp:v1"))
}
func (s *SecondFactor) Verify(encrypted []byte, code string, now time.Time) (int64, error) {
	if len(code) != 6 || len(encrypted) < s.aead.NonceSize()+s.aead.Overhead() {
		return 0, domain.ErrUnauthenticated
	}
	n := s.aead.NonceSize()
	secret, err := s.aead.Open(nil, encrypted[:n], encrypted[n:], []byte("nsm:admin-totp:v1"))
	if err != nil {
		return 0, domain.ErrUnauthenticated
	}
	// Return the validated counter so the transactional adapter can reject reuse.
	for _, offset := range []int64{0, -1, 1} {
		t := now.Add(time.Duration(offset) * 30 * time.Second)
		expected, err := totp.GenerateCode(string(secret), t)
		if err == nil && subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			return t.Unix() / 30, nil
		}
	}
	return 0, domain.ErrUnauthenticated
}
