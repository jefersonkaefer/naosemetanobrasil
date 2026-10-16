//go:build !e2e

package main

import (
	"naosemeta.com/backend/internal/adapters/instagram"
	"naosemeta.com/backend/internal/application"
	"naosemeta.com/backend/internal/config"
)

func instagramProfiles(cfg config.Config) application.InstagramProfiles {
	return instagram.NewProfiles(cfg.MetaAccessToken, cfg.MetaAPIVersion)
}
