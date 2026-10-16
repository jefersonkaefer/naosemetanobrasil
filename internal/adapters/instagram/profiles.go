package instagram

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"naosemeta.com/backend/internal/domain"
	"net/http"
	"regexp"
	"time"
)

var graphVersion = regexp.MustCompile(`^v[1-9][0-9]?\.[0-9]{1,2}$`)
var errProfile = errors.New("instagram identity lookup unavailable")

type Profiles struct {
	token, version string
	client         *http.Client
}

func NewProfiles(token, version string) *Profiles {
	return &Profiles{token: token, version: version, client: &http.Client{Timeout: 4 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (p *Profiles) Handle(ctx context.Context, senderID string) (string, error) {
	if !numericID.MatchString(senderID) || !graphVersion.MatchString(p.version) || p.token == "" {
		return "", errProfile
	}
	// Fix the origin so no webhook field can exfiltrate the bearer token.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://graph.instagram.com/"+p.version+"/"+senderID+"?fields=id,username", nil)
	if err != nil {
		return "", errProfile
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	res, err := p.client.Do(req)
	if err != nil {
		return "", errProfile
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", errProfile
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, (64<<10)+1))
	if err != nil || len(body) > 64<<10 {
		return "", errProfile
	}
	var profile struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	}
	if json.Unmarshal(body, &profile) != nil || profile.ID != senderID {
		return "", errProfile
	}
	handle, err := domain.NormalizeHandle(profile.Username)
	if err != nil {
		return "", errProfile
	}
	return handle, nil
}
