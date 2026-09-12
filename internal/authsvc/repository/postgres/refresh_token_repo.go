package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/authsvc/domain"
)

type RefreshTokenRepository struct {
	pool *pgxpool.Pool
}

func NewRefreshTokenRepository(pool *pgxpool.Pool) *RefreshTokenRepository {
	return &RefreshTokenRepository{pool: pool}
}

func (r *RefreshTokenRepository) Create(ctx context.Context, t *domain.RefreshToken) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO auth.refresh_tokens (id, user_id, org_id, token_hash, issued_at, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		t.ID, t.UserID, t.OrgID, t.TokenHash, t.IssuedAt, t.ExpiresAt,
	)
	return err
}

func (r *RefreshTokenRepository) GetByHash(ctx context.Context, orgID, tokenHash string) (*domain.RefreshToken, error) {
	var t domain.RefreshToken
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, org_id, token_hash, issued_at, expires_at, revoked_at, replaced_by
		 FROM auth.refresh_tokens WHERE token_hash = $1 AND org_id = $2`,
		tokenHash, orgID,
	).Scan(&t.ID, &t.UserID, &t.OrgID, &t.TokenHash, &t.IssuedAt, &t.ExpiresAt, &t.RevokedAt, &t.ReplacedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrInvalidRefresh
	}
	return &t, err
}

func (r *RefreshTokenRepository) Revoke(ctx context.Context, orgID, tokenID string, replacedBy *string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE auth.refresh_tokens SET revoked_at = now(), replaced_by = $3
		 WHERE id = $1 AND org_id = $2 AND revoked_at IS NULL`,
		tokenID, orgID, replacedBy,
	)
	return err
}
