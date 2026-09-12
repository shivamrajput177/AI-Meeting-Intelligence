package usecase

import (
	"context"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/authsvc/domain"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/jwtutil"
)

type RefreshInput struct {
	// OrgID is required because refresh tokens are looked up scoped to an
	// org (see RefreshTokenRepository) — the client must have kept it
	// from its last login/signup response, alongside the refresh token
	// itself.
	OrgID        string
	RefreshToken string
	// Role is carried by the client from its last token pair. Phase 1
	// doesn't re-derive it from User Service on every refresh (that would
	// mean a synchronous call to another service on every refresh, for a
	// value that essentially never changes between two refreshes) — a
	// role change (Phase 2's PATCH .../role) takes effect on the user's
	// *next* login/refresh cycle once they present a stale role, which is
	// an acceptable staleness window, not silently ignored forever.
	Role string
}

type RefreshUseCase struct {
	refreshRepo domain.RefreshTokenRepository
	tokenIssuer *TokenIssuer
}

func NewRefreshUseCase(refreshRepo domain.RefreshTokenRepository, tokenIssuer *TokenIssuer) *RefreshUseCase {
	return &RefreshUseCase{refreshRepo: refreshRepo, tokenIssuer: tokenIssuer}
}

func (uc *RefreshUseCase) Execute(ctx context.Context, in RefreshInput) (*TokenPair, error) {
	if in.OrgID == "" || in.RefreshToken == "" {
		return nil, apperr.BadRequest("orgId and refreshToken are required")
	}

	hash := jwtutil.HashRefreshToken(in.RefreshToken)
	token, err := uc.refreshRepo.GetByHash(ctx, in.OrgID, hash)
	if err != nil {
		return nil, domain.ErrInvalidRefresh
	}
	if token.RevokedAt != nil {
		// A revoked token being presented again means either the client
		// raced two refreshes, or — the case worth logging loudly in a
		// real deployment — the token was stolen and both the attacker
		// and the legitimate holder are trying to use it. Phase 1 just
		// rejects; Phase 6's audit trail is where this becomes an alert.
		return nil, domain.ErrInvalidRefresh
	}
	if token.ExpiresAt.Before(time.Now()) {
		return nil, domain.ErrInvalidRefresh
	}

	return uc.tokenIssuer.Rotate(ctx, token.UserID, token.OrgID, in.Role, token.ID)
}
