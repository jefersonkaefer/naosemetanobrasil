package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"naosemeta.com/backend/internal/application"
	"naosemeta.com/backend/internal/domain"
)

type Limiter struct {
	poolStore *Store
	secrets   application.Secrets
}

func NewLimiter(store *Store, secrets application.Secrets) *Limiter { return &Limiter{store, secrets} }
func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration, now time.Time) error {
	var count int
	start := now.UTC().Truncate(window)
	err := l.poolStore.pool.QueryRow(ctx, `INSERT INTO app.rate_limit_buckets(key_hash,window_start,count,expires_at) VALUES($1,$2,1,$3) ON CONFLICT(key_hash,window_start) DO UPDATE SET count=app.rate_limit_buckets.count+1 WHERE app.rate_limit_buckets.count<$4 RETURNING count`, l.secrets.Digest("rate:"+key), start, start.Add(window), limit).Scan(&count)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrLimited
	}
	return err
}

// Cleanup is bounded and can run on genuine API activity; correctness never depends on it.
func (s *Store) Cleanup(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM app.rate_limit_buckets WHERE ctid IN (SELECT ctid FROM app.rate_limit_buckets WHERE expires_at<now()-interval '1 day' LIMIT 500)`)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM app.sessions WHERE id IN (SELECT id FROM app.sessions WHERE expires_at<now()-interval '1 day' LIMIT 500)`)
	return err
}
