package usecase

import (
	"context"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/auth-service/domain"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/auth-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/jwtutil"
)

type RefreshUseCase struct {
	refreshRepo domain.RefreshTokenRepository
	tokenIssuer *TokenIssuer
}

func NewRefreshUseCase(refreshRepo domain.RefreshTokenRepository, tokenIssuer *TokenIssuer) *RefreshUseCase {
	return &RefreshUseCase{refreshRepo: refreshRepo, tokenIssuer: tokenIssuer}
}

func (uc *RefreshUseCase) Refresh(ctx context.Context, in entity.RefreshInput) (*entity.TokenPair, error) {
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
