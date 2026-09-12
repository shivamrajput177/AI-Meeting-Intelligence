// Package postgres implements authsvc/domain's repositories against the
// auth.* schema (see docs/architecture/database-schema.md). Unlike
// user.users/meeting.meetings/etc., these tables have no Row-Level
// Security policy in the schema design — they're only ever queried by a
// specific user_id or a secret token hash, never listed broadly by org,
// so the usual cross-tenant-leak risk RLS defends against doesn't apply
// the same way here. Every query still filters by org_id explicitly as a
// defense-in-depth belt-and-braces check, even without DB-enforced RLS.
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/authsvc/domain"
)

const (
	maxFailedAttempts = 5
	lockoutDuration   = 15 * time.Minute
)

type CredentialsRepository struct {
	pool *pgxpool.Pool
}

func NewCredentialsRepository(pool *pgxpool.Pool) *CredentialsRepository {
	return &CredentialsRepository{pool: pool}
}

func (r *CredentialsRepository) Create(ctx context.Context, c *domain.Credentials) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO auth.credentials (user_id, org_id, password_hash, algo, updated_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		c.UserID, c.OrgID, c.PasswordHash, c.Algo, c.UpdatedAt,
	)
	return err
}

func (r *CredentialsRepository) GetByUserID(ctx context.Context, orgID, userID string) (*domain.Credentials, error) {
	var c domain.Credentials
	err := r.pool.QueryRow(ctx,
		`SELECT user_id, org_id, password_hash, algo, failed_attempts, locked_until, updated_at
		 FROM auth.credentials WHERE user_id = $1 AND org_id = $2`,
		userID, orgID,
	).Scan(&c.UserID, &c.OrgID, &c.PasswordHash, &c.Algo, &c.FailedAttempts, &c.LockedUntil, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrInvalidCredentials
	}
	return &c, err
}

func (r *CredentialsRepository) IncrementFailedAttempts(ctx context.Context, orgID, userID string) error {
	// make_interval(secs => ...) sidesteps any ambiguity between Go's
	// time.Duration string format ("15m0s") and Postgres's interval
	// literal grammar — a plain numeric numbers-of-seconds argument has
	// exactly one interpretation.
	_, err := r.pool.Exec(ctx,
		`UPDATE auth.credentials
		 SET failed_attempts = failed_attempts + 1,
		     locked_until = CASE WHEN failed_attempts + 1 >= $3 THEN now() + make_interval(secs => $4) ELSE locked_until END,
		     updated_at = now()
		 WHERE user_id = $1 AND org_id = $2`,
		userID, orgID, maxFailedAttempts, lockoutDuration.Seconds(),
	)
	return err
}

func (r *CredentialsRepository) ResetFailedAttempts(ctx context.Context, orgID, userID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE auth.credentials SET failed_attempts = 0, locked_until = NULL, updated_at = now()
		 WHERE user_id = $1 AND org_id = $2`,
		userID, orgID,
	)
	return err
}

func (r *CredentialsRepository) UpdatePassword(ctx context.Context, orgID, userID, passwordHash string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE auth.credentials SET password_hash = $3, failed_attempts = 0, locked_until = NULL, updated_at = now()
		 WHERE user_id = $1 AND org_id = $2`,
		userID, orgID, passwordHash,
	)
	return err
}
