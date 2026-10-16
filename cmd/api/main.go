package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"naosemeta.com/backend/internal/adapters/httpapi"
	"naosemeta.com/backend/internal/adapters/instagram"
	"naosemeta.com/backend/internal/adapters/postgres"
	"naosemeta.com/backend/internal/adapters/security"
	"naosemeta.com/backend/internal/adapters/webui"
	"naosemeta.com/backend/internal/application"
	"naosemeta.com/backend/internal/config"
)

func main() {
	if err := run(); err != nil {
		slog.Error("API initialization failed; verify configuration and dependencies")
		os.Exit(1)
	}
}
func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "reason", err.Error())
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	pc, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	pc.MaxConns = cfg.MaxConnections
	pc.MinConns = 0
	pc.ConnConfig.ConnectTimeout = 5 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return err
	}
	defer pool.Close()
	// Liveness remains available during a database outage; readiness reports it.
	store := postgres.New(pool)
	secrets := security.NewSecrets(cfg.Pepper)
	clock := security.Clock{}
	passwords, err := security.NewPasswords()
	if err != nil {
		return err
	}
	second, err := security.NewSecondFactor(cfg.TOTPKey)
	if err != nil {
		return err
	}
	limiter := postgres.NewLimiter(store, secrets)
	auth := application.NewAuth(store, store, passwords, second, secrets, clock)
	verification := application.NewVerification(store, limiter, secrets, clock, cfg.OfficialHandle)
	// os.Root confines static-file resolution even when an asset is a symlink.
	var frontend http.Handler
	root, err := os.OpenRoot(cfg.FrontendDir)
	if err == nil {
		defer root.Close()
		frontend, err = webui.New(root.FS())
	}
	if err != nil {
		if cfg.Environment == "production" {
			return errors.New("frontend build unavailable")
		}
		logger.Warn("frontend build unavailable; serving API only")
	}
	var webhook http.Handler
	if cfg.InstagramMode == "webhook" {
		profiles := instagramProfiles(cfg)
		receiver := application.NewInstagramVerification(store, profiles, secrets, clock)
		webhook = instagram.NewWebhook(cfg.MetaAppSecret, cfg.MetaVerifyToken, cfg.MetaAccountID, receiver)
	}
	handler := httpapi.New(auth, verification, limiter, clock, secrets, httpapi.Options{Origin: cfg.Origin, SecureCookies: cfg.Environment == "production", TrustedProxies: cfg.TrustedProxies, Logger: logger, Ready: store.Ready, Cleanup: store.Cleanup, Frontend: frontend, InstagramWebhook: webhook, InstagramMode: cfg.InstagramMode})
	server := &http.Server{Addr: net.JoinHostPort(cfg.ListenHost, cfg.Port), Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024}
	errCh := make(chan error, 1)
	go func() { logger.Info("API listening", "port", cfg.Port); errCh <- server.ListenAndServe() }()
	select {
	case err = <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
	return nil
}
