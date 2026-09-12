package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/authsvc/domain"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/jwtutil"
)

// TokenPair is what every route that "logs someone in" (signup, login,
// refresh) returns.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int // seconds, for the client to know when to refresh
}

// TokenIssuer centralizes access+refresh token creation so signup, login,
// and refresh all produce tokens the exact same way — see
// docs/PROJECT_PLAN.md §5 for the access/refresh TTL and rotation design.
type TokenIssuer struct {
	secret      []byte
	accessTTL   time.Duration
	refreshTTL  time.Duration
	refreshRepo domain.RefreshTokenRepository
}

func NewTokenIssuer(secret []byte, accessTTL, refreshTTL time.Duration, refreshRepo domain.RefreshTokenRepository) *TokenIssuer {
	return &TokenIssuer{secret: secret, accessTTL: accessTTL, refreshTTL: refreshTTL, refreshRepo: refreshRepo}
}

// Issue mints a fresh access+refresh pair with no prior refresh token to
// revoke (signup, login).
func (ti *TokenIssuer) Issue(ctx context.Context, userID, orgID, role string) (*TokenPair, error) {
	return ti.issue(ctx, userID, orgID, role, nil)
}

// Rotate mints a fresh pair AND revokes replacing the refresh token whose
// id is oldRefreshID, linking replaced_by so replay of the old token is
// detectable (see docs/architecture/observability-security.md §2).
func (ti *TokenIssuer) Rotate(ctx context.Context, userID, orgID, role, oldRefreshID string) (*TokenPair, error) {
	return ti.issue(ctx, userID, orgID, role, &oldRefreshID)
}

func (ti *TokenIssuer) issue(ctx context.Context, userID, orgID, role string, revoke *string) (*TokenPair, error) {
	accessToken, _, err := jwtutil.GenerateAccessToken(ti.secret, userID, orgID, role, ti.accessTTL)
	if err != nil {
		return nil, apperr.Internal("issue access token").Wrap(err)
	}

	rawRefresh, hash, err := jwtutil.NewOpaqueRefreshToken()
	if err != nil {
		return nil, apperr.Internal("issue refresh token").Wrap(err)
	}
	newID := uuid.NewString()
	now := time.Now()
	if err := ti.refreshRepo.Create(ctx, &domain.RefreshToken{
		ID: newID, UserID: userID, OrgID: orgID, TokenHash: hash,
		IssuedAt: now, ExpiresAt: now.Add(ti.refreshTTL),
	}); err != nil {
		return nil, apperr.Internal("store refresh token").Wrap(err)
	}

	if revoke != nil {
		if err := ti.refreshRepo.Revoke(ctx, orgID, *revoke, &newID); err != nil {
			return nil, apperr.Internal("revoke previous refresh token").Wrap(err)
		}
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresIn:    int(ti.accessTTL.Seconds()),
	}, nil
}
