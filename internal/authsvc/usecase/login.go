package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/authsvc/domain"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/passwordutil"
)

type LoginInput struct {
	Email    string
	Password string
}

type LoginUseCase struct {
	userClient  domain.UserClient
	credentials domain.CredentialsRepository
	tokenIssuer *TokenIssuer
}

func NewLoginUseCase(userClient domain.UserClient, credentials domain.CredentialsRepository, tokenIssuer *TokenIssuer) *LoginUseCase {
	return &LoginUseCase{userClient: userClient, credentials: credentials, tokenIssuer: tokenIssuer}
}

// Execute resolves which org an email belongs to via User Service (see
// domain.UserClient.LookupByEmail and its "multi-org-per-email" trade-off
// note), then verifies the password against Auth Service's own
// credentials table.
func (uc *LoginUseCase) Execute(ctx context.Context, in LoginInput) (*TokenPair, error) {
	email := strings.TrimSpace(strings.ToLower(in.Email))
	if email == "" || in.Password == "" {
		return nil, apperr.BadRequest("email and password are required")
	}

	matches, err := uc.userClient.LookupByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, domain.ErrInvalidCredentials
	}
	// Phase 1 has no invite flow, so more than one active account sharing
	// an email is the rare case of someone signing up twice independently
	// — pick the first (oldest) match rather than exposing an
	// org-disambiguation step the product doesn't have a UI for yet.
	match := matches[0]

	creds, err := uc.credentials.GetByUserID(ctx, match.OrgID, match.UserID)
	if err != nil {
		return nil, domain.ErrInvalidCredentials
	}
	if creds.LockedUntil != nil && creds.LockedUntil.After(time.Now()) {
		return nil, domain.ErrAccountLocked
	}

	ok, err := passwordutil.Verify(in.Password, creds.PasswordHash)
	if err != nil || !ok {
		_ = uc.credentials.IncrementFailedAttempts(ctx, match.OrgID, match.UserID)
		return nil, domain.ErrInvalidCredentials
	}

	_ = uc.credentials.ResetFailedAttempts(ctx, match.OrgID, match.UserID)
	return uc.tokenIssuer.Issue(ctx, match.UserID, match.OrgID, match.Role)
}
