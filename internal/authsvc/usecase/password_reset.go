package usecase

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/authsvc/domain"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/jwtutil"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/passwordutil"
)

const resetTokenTTL = 1 * time.Hour

// RequestPasswordResetUseCase creates a reset token. There is no
// Notification Service yet to email it (that's Phase 4 — see
// docs/architecture/microservices.md §10), so Phase 1 logs the raw token
// server-side and, only when devExposeToken is true (an explicit local-dev
// flag, never set in the public demo deployment), returns it in the
// response so the flow is testable end-to-end without a real mailbox.
type RequestPasswordResetUseCase struct {
	userClient     domain.UserClient
	resetRepo      domain.PasswordResetRepository
	log            *slog.Logger
	devExposeToken bool
}

func NewRequestPasswordResetUseCase(userClient domain.UserClient, resetRepo domain.PasswordResetRepository, log *slog.Logger, devExposeToken bool) *RequestPasswordResetUseCase {
	return &RequestPasswordResetUseCase{userClient: userClient, resetRepo: resetRepo, log: log, devExposeToken: devExposeToken}
}

// Execute always returns success (never reveals whether the email exists
// — a standard defense against account enumeration), and a dev-only token
// string that's empty unless devExposeToken is set.
func (uc *RequestPasswordResetUseCase) Execute(ctx context.Context, email string) (devToken string, err error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return "", apperr.BadRequest("email is required")
	}

	matches, lookupErr := uc.userClient.LookupByEmail(ctx, email)
	if lookupErr != nil || len(matches) == 0 {
		return "", nil // deliberately silent — see doc comment
	}
	userID, orgID := matches[0].UserID, matches[0].OrgID

	raw, hash, genErr := jwtutil.NewOpaqueRefreshToken() // same primitive: random opaque token
	if genErr != nil {
		return "", apperr.Internal("generate reset token").Wrap(genErr)
	}

	if err := uc.resetRepo.Create(ctx, &domain.PasswordResetToken{
		ID: uuid.NewString(), UserID: userID, OrgID: orgID, TokenHash: hash,
		ExpiresAt: time.Now().Add(resetTokenTTL),
	}); err != nil {
		return "", apperr.Internal("store reset token").Wrap(err)
	}

	uc.log.Info("password reset requested",
		slog.String("user_id", userID), slog.String("dev_reset_token", raw))

	if uc.devExposeToken {
		return raw, nil
	}
	return "", nil
}

type ConfirmPasswordResetUseCase struct {
	resetRepo   domain.PasswordResetRepository
	credentials domain.CredentialsRepository
}

func NewConfirmPasswordResetUseCase(resetRepo domain.PasswordResetRepository, credentials domain.CredentialsRepository) *ConfirmPasswordResetUseCase {
	return &ConfirmPasswordResetUseCase{resetRepo: resetRepo, credentials: credentials}
}

func (uc *ConfirmPasswordResetUseCase) Execute(ctx context.Context, token, newPassword string) error {
	if token == "" || len(newPassword) < 8 {
		return apperr.BadRequest("token is required and password must be at least 8 characters")
	}

	hash := jwtutil.HashRefreshToken(token)
	rt, err := uc.resetRepo.GetByHash(ctx, hash)
	if err != nil || rt.UsedAt != nil || rt.ExpiresAt.Before(time.Now()) {
		return domain.ErrInvalidResetToken
	}

	newHash, err := passwordutil.Hash(newPassword)
	if err != nil {
		return apperr.Internal("hash password").Wrap(err)
	}
	if err := uc.credentials.UpdatePassword(ctx, rt.OrgID, rt.UserID, newHash); err != nil {
		return apperr.Internal("update password").Wrap(err)
	}
	return uc.resetRepo.MarkUsed(ctx, rt.ID)
}
