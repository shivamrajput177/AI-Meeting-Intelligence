// Package dbx wraps the Postgres connection pool with two things every
// service needs: a migration runner (so `docker compose up` alone gets a
// schema-per-service database into shape, per
// docs/architecture/database-schema.md) and a per-transaction tenant
// context helper (the app-side half of the Row-Level Security story in
// docs/architecture/database-schema.md §"Row-Level Security pattern").
package dbx

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool opens a connection pool against connString, retrying briefly —
// docker-compose starts Postgres and every service roughly together, and
// there's no init-container/health-gate wiring in Phase 1's compose file,
// so a service coming up before Postgres accepts connections is the
// common case, not the exception.
func NewPool(ctx context.Context, connString string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("dbx: parse config: %w", err)
	}
	// SimpleProtocol lets Exec run a migration file's multiple
	// semicolon-separated statements in one round trip — the extended
	// protocol pgx defaults to (prepare + bind) rejects multi-statement
	// strings outright.
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	var pool *pgxpool.Pool
	var lastErr error
	for attempt := 1; attempt <= 10; attempt++ {
		pool, lastErr = pgxpool.NewWithConfig(ctx, cfg)
		if lastErr == nil {
			if lastErr = pool.Ping(ctx); lastErr == nil {
				return pool, nil
			}
			pool.Close()
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt) * 500 * time.Millisecond):
		}
	}
	return nil, fmt.Errorf("dbx: could not connect after retries: %w", lastErr)
}
