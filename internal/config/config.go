package config

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"naosemeta.com/backend/internal/domain"
)

type Config struct {
	Environment     string
	Port            string
	ListenHost      string
	DatabaseURL     string
	MaxConnections  int32
	Origin          string
	OfficialHandle  string
	Pepper          string
	TOTPKey         []byte
	TrustedProxies  []netip.Prefix
	FrontendDir     string
	InstagramMode   string
	MetaAppSecret   string
	MetaVerifyToken string
	MetaAccessToken string
	MetaAccountID   string
	MetaAPIVersion  string
}

func Load() (Config, error) { return Parse(os.Getenv) }
func Parse(get func(string) string) (Config, error) {
	c := Config{Environment: get("APP_ENV"), Port: get("PORT"), DatabaseURL: get("DATABASE_URL"), Origin: get("PUBLIC_BASE_URL"), Pepper: get("VERIFICATION_CODE_PEPPER"), MaxConnections: 5}
	c.FrontendDir = get("FRONTEND_DIR")
	if c.FrontendDir == "" {
		c.FrontendDir = "web/dist"
	}
	if c.Environment == "" {
		c.Environment = "production"
	}
	if c.Environment != "production" && c.Environment != "development" {
		return c, fmt.Errorf("APP_ENV must be production or development")
	}
	// Render provides the actual HTTPS service URL, including any generated
	// suffix. An explicit custom domain always takes precedence.
	if c.Origin == "" && c.Environment == "production" {
		c.Origin = get("RENDER_EXTERNAL_URL")
	}
	c.ListenHost = get("LISTEN_HOST")
	if c.ListenHost == "" {
		c.ListenHost = "0.0.0.0"
		if c.Environment == "development" {
			c.ListenHost = "127.0.0.1"
		}
	}
	if _, err := netip.ParseAddr(c.ListenHost); err != nil {
		return c, fmt.Errorf("LISTEN_HOST must be an IP address")
	}
	if c.Port == "" {
		c.Port = "8080"
	}
	port, err := strconv.Atoi(c.Port)
	if err != nil || port < 1 || port > 65535 {
		return c, fmt.Errorf("invalid PORT")
	}
	if value := get("DATABASE_MAX_CONNECTIONS"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 5 {
			return c, fmt.Errorf("DATABASE_MAX_CONNECTIONS must be between 1 and 5")
		}
		c.MaxConnections = int32(n)
	}
	if err = ValidateDatabaseURL(c.DatabaseURL, c.Environment); err != nil {
		return c, err
	}
	u, err := url.Parse(c.Origin)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return c, fmt.Errorf("PUBLIC_BASE_URL must be a single origin")
	}
	if u.Scheme != "https" && (c.Environment != "development" || u.Scheme != "http" || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1")) {
		return c, fmt.Errorf("PUBLIC_BASE_URL requires HTTPS outside localhost development")
	}
	c.Origin = strings.TrimSuffix(c.Origin, "/")
	c.OfficialHandle, err = domain.NormalizeHandle(get("INSTAGRAM_OFFICIAL_HANDLE"))
	if err != nil {
		return c, fmt.Errorf("invalid INSTAGRAM_OFFICIAL_HANDLE")
	}
	c.InstagramMode = get("INSTAGRAM_VERIFICATION_MODE")
	if c.InstagramMode == "" {
		c.InstagramMode = "manual"
	}
	if c.InstagramMode != "manual" && c.InstagramMode != "webhook" {
		return c, fmt.Errorf("invalid INSTAGRAM_VERIFICATION_MODE")
	}
	if c.InstagramMode == "webhook" {
		c.MetaAppSecret, c.MetaVerifyToken, c.MetaAccessToken = get("META_APP_SECRET"), get("META_WEBHOOK_VERIFY_TOKEN"), get("INSTAGRAM_ACCESS_TOKEN")
		c.MetaAccountID, c.MetaAPIVersion = get("INSTAGRAM_ACCOUNT_ID"), get("META_GRAPH_API_VERSION")
		if len(c.MetaAppSecret) < 32 || len(c.MetaVerifyToken) < 32 || len(c.MetaAccessToken) < 20 || strings.HasPrefix(c.MetaAppSecret, "replace-") || strings.HasPrefix(c.MetaVerifyToken, "replace-") || strings.HasPrefix(c.MetaAccessToken, "replace-") || strings.ContainsAny(c.MetaAccessToken, " \r\n\t") || !regexp.MustCompile(`^[0-9]{1,32}$`).MatchString(c.MetaAccountID) || !regexp.MustCompile(`^v[1-9][0-9]?\.[0-9]{1,2}$`).MatchString(c.MetaAPIVersion) {
			return c, fmt.Errorf("webhook mode requires valid Meta credentials, account ID and explicit Graph API version")
		}
		if sameKeyMaterial(c.MetaVerifyToken, []byte(c.MetaAppSecret)) || sameKeyMaterial(c.MetaAppSecret, []byte(c.MetaVerifyToken)) {
			return c, fmt.Errorf("Meta app secret and webhook verify token must be independent")
		}
		if c.Environment == "production" && c.MetaAccessToken == "synthetic-e2e-token-never-use-in-production" {
			return c, fmt.Errorf("synthetic Meta token is forbidden in production")
		}
	}
	if len(c.Pepper) < 32 || strings.HasPrefix(c.Pepper, "replace-") || (c.Environment == "production" && strings.HasPrefix(c.Pepper, "local-development-")) {
		return c, fmt.Errorf("VERIFICATION_CODE_PEPPER requires at least 32 random bytes")
	}
	c.TOTPKey, err = base64.StdEncoding.DecodeString(get("TOTP_ENCRYPTION_KEY"))
	if err != nil || len(c.TOTPKey) != 32 {
		return c, fmt.Errorf("TOTP_ENCRYPTION_KEY must encode 32 bytes in base64")
	}
	if c.Environment == "production" && strings.Trim(string(c.TOTPKey), "\x00") == "" {
		return c, fmt.Errorf("TOTP_ENCRYPTION_KEY cannot use the development key in production")
	}
	if c.Environment == "production" && sameKeyMaterial(c.Pepper, c.TOTPKey) {
		return c, fmt.Errorf("pepper and TOTP encryption key must be independent")
	}
	for _, raw := range strings.Split(get("TRUSTED_PROXY_CIDRS"), ",") {
		if raw = strings.TrimSpace(raw); raw == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(raw)
		if err != nil || prefix.Bits() == 0 {
			return c, fmt.Errorf("invalid or unrestricted TRUSTED_PROXY_CIDRS")
		}
		c.TrustedProxies = append(c.TrustedProxies, prefix)
	}
	return c, nil
}

func ValidateDatabaseURL(raw, environment string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User == nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return fmt.Errorf("DATABASE_URL must be a PostgreSQL URL")
	}
	if environment != "development" {
		query, err := url.ParseQuery(u.RawQuery)
		if err != nil || len(query["sslmode"]) != 1 || query.Get("sslmode") != "verify-full" {
			return fmt.Errorf("DATABASE_URL requires unambiguous sslmode=verify-full in production")
		}
		// pgx supports aliases and overrides that net/url.Query().Get does not
		// interpret the same way, including ssl=true and repeated sslmode keys.
		// Keep destination and credentials in the authority, never in query.
		allowed := map[string]bool{"sslmode": true, "sslrootcert": true, "sslcert": true, "sslkey": true, "connect_timeout": true, "application_name": true, "default_query_exec_mode": true}
		for key, values := range query {
			if !allowed[key] || len(values) != 1 {
				return fmt.Errorf("DATABASE_URL contains unsupported or repeated connection options")
			}
		}
	}
	return nil
}

func sameKeyMaterial(pepper string, key []byte) bool {
	if pepper == string(key) {
		return true
	}
	for _, decode := range []func(string) ([]byte, error){hex.DecodeString, base64.StdEncoding.DecodeString, base64.RawStdEncoding.DecodeString, base64.URLEncoding.DecodeString, base64.RawURLEncoding.DecodeString} {
		if decoded, err := decode(pepper); err == nil && string(decoded) == string(key) {
			return true
		}
	}
	return false
}
