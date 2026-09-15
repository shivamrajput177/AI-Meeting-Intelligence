// Package repository defines the interfaces usecase depends on;
// repository/postgres, right below this package in the same tree,
// implements them. Keeping the interfaces here instead of off in some
// unrelated package is just where they belong — their one real
// implementation lives one directory down. usecase still only ever
// depends on these interface types, never on the concrete
// *postgres.CredentialsRepository/etc. directly, which is what lets it
// be unit-tested against an in-memory fake with zero Postgres involved.
package repository

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/auth-service/entity"
)

type CredentialsRepository interface {
	Create(ctx context.Context, c *entity.Credentials) error
	GetByUserID(ctx context.Context, orgID, userID string) (*entity.Credentials, error)
	IncrementFailedAttempts(ctx context.Context, orgID, userID string) error
	ResetFailedAttempts(ctx context.Context, orgID, userID string) error
	UpdatePassword(ctx context.Context, orgID, userID, passwordHash string) error
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, t *entity.RefreshToken) error
	GetByHash(ctx context.Context, orgID, tokenHash string) (*entity.RefreshToken, error)
	Revoke(ctx context.Context, orgID, tokenID string, replacedBy *string) error
}

type PasswordResetRepository interface {
	Create(ctx context.Context, t *entity.PasswordResetToken) error
	GetByHash(ctx context.Context, tokenHash string) (*entity.PasswordResetToken, error)
	MarkUsed(ctx context.Context, id string) error
}
