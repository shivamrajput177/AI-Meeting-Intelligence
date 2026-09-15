package domain

import "context"

type CredentialsRepository interface {
	Create(ctx context.Context, c *Credentials) error
	GetByUserID(ctx context.Context, orgID, userID string) (*Credentials, error)
	IncrementFailedAttempts(ctx context.Context, orgID, userID string) error
	ResetFailedAttempts(ctx context.Context, orgID, userID string) error
	UpdatePassword(ctx context.Context, orgID, userID, passwordHash string) error
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, t *RefreshToken) error
	GetByHash(ctx context.Context, orgID, tokenHash string) (*RefreshToken, error)
	Revoke(ctx context.Context, orgID, tokenID string, replacedBy *string) error
}

type PasswordResetRepository interface {
	Create(ctx context.Context, t *PasswordResetToken) error
	GetByHash(ctx context.Context, tokenHash string) (*PasswordResetToken, error)
	MarkUsed(ctx context.Context, id string) error
}
