package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"naosemeta.com/backend/internal/config"
	"naosemeta.com/backend/migrations"
)

func main() {
	if err := run(); err != nil {
		log.Print("Migration failed; check the database and migration history.")
		os.Exit(1)
	}
	log.Print("Migrations applied.")
}
func run() error {
	raw := os.Getenv("DATABASE_URL")
	if err := config.ValidateDatabaseURL(raw, os.Getenv("APP_ENV")); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, raw)
	if err != nil {
		return err
	}
	defer pool.Close()
	return migrations.Up(ctx, pool)
}
