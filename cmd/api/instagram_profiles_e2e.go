//go:build e2e

package main

import (
	"context"
	"errors"
	"naosemeta.com/backend/internal/application"
	"naosemeta.com/backend/internal/config"
	"net/url"
	"strings"
)

// Compiled only into the local E2E binary. Production Docker builds do not
// include this implementation, and no environment flag can enable it there.
type syntheticProfiles struct{ enabled bool }

func (p syntheticProfiles) Handle(_ context.Context, id string) (string, error) {
	if !p.enabled || len(id) > 16 {
		return "", errors.New("synthetic profile lookup disabled")
	}
	return "perfil_e2e_" + id, nil
}
func instagramProfiles(cfg config.Config) application.InstagramProfiles {
	u, err := url.Parse(cfg.DatabaseURL)
	enabled := err == nil && cfg.Environment == "development" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost" || u.Hostname() == "::1") && strings.HasSuffix(u.Path, "_test") && cfg.MetaAccessToken == "synthetic-e2e-token-never-use-in-production"
	return syntheticProfiles{enabled}
}
