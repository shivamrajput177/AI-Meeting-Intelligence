package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/auth-service/client"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/auth-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/auth-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/passwordutil"
)

type LoginUseCase struct {
	userClient  client.UserClient
	credentials repository.CredentialsRepository
	tokenIssuer *TokenIssuer
}

func NewLoginUseCase(userClient client.UserClient, credentials repository.CredentialsRepository, tokenIssuer *TokenIssuer) *LoginUseCase {
	return &LoginUseCase{userClient: userClient, credentials: credentials, tokenIssuer: tokenIssuer}
}

// Login resolves which org an email belongs to via User Service (see
// client.UserClient.LookupByEmail and its "multi-org-per-email" trade-off
// note), then verifies the password against Auth Service's own
// credentials table.
func (uc *LoginUseCase) Login(ctx context.Context, in entity.LoginInput) (*entity.TokenPair, error) {
	email := strings.TrimSpace(strings.ToLower(in.Email))
	if email == "" || in.Password == "" {
		return nil, apperr.BadRequest("email and password are required")
	}

	matches, err := uc.userClient.LookupByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, apperr.Unauthorized("invalid email or password")
	}
	// Phase 1 has no invite flow, so more than one active account sharing
	// an email is the rare case of someone signing up twice independently
	// — pick the first (oldest) match rather than exposing an
	// org-disambiguation step the product doesn't have a UI for yet.
	match := matches[0]

	creds, err := uc.credentials.GetByUserID(ctx, match.OrgID, match.UserID)
	if err != nil {
		return nil, apperr.Unauthorized("invalid email or password")
	}
	if creds.LockedUntil != nil && creds.LockedUntil.After(time.Now()) {
		return nil, apperr.Unauthorized("account temporarily locked, try again later")
	}

	ok, err := passwordutil.Verify(in.Password, creds.PasswordHash)
	if err != nil || !ok {
		_ = uc.credentials.IncrementFailedAttempts(ctx, match.OrgID, match.UserID)
		return nil, apperr.Unauthorized("invalid email or password")
	}

	_ = uc.credentials.ResetFailedAttempts(ctx, match.OrgID, match.UserID)
	return uc.tokenIssuer.Issue(ctx, match.UserID, match.OrgID, match.Role)
}
