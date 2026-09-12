package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/authsvc/domain"
)

type PasswordResetRepository struct {
	pool *pgxpool.Pool
}

func NewPasswordResetRepository(pool *pgxpool.Pool) *PasswordResetRepository {
	return &PasswordResetRepository{pool: pool}
}

func (r *PasswordResetRepository) Create(ctx context.Context, t *domain.PasswordResetToken) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO auth.password_reset_tokens (id, user_id, org_id, token_hash, expires_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		t.ID, t.UserID, t.OrgID, t.TokenHash, t.ExpiresAt,
	)
	return err
}

func (r *PasswordResetRepository) GetByHash(ctx context.Context, tokenHash string) (*domain.PasswordResetToken, error) {
	var t domain.PasswordResetToken
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, org_id, token_hash, expires_at, used_at
		 FROM auth.password_reset_tokens WHERE token_hash = $1`,
		tokenHash,
	).Scan(&t.ID, &t.UserID, &t.OrgID, &t.TokenHash, &t.ExpiresAt, &t.UsedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrInvalidResetToken
	}
	return &t, err
}

func (r *PasswordResetRepository) MarkUsed(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE auth.password_reset_tokens SET used_at = now() WHERE id = $1`,
		id,
	)
	return err
}
