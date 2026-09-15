package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/auth-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/auth-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/jwtutil"
)

type LogoutUseCase struct {
	refreshRepo repository.RefreshTokenRepository
}

func NewLogoutUseCase(refreshRepo repository.RefreshTokenRepository) *LogoutUseCase {
	return &LogoutUseCase{refreshRepo: refreshRepo}
}

// Logout revokes the presented refresh token. It intentionally succeeds
// even if the token is already gone/invalid — logout is idempotent from
// the client's point of view ("am I logged out now?" — yes, either way).
func (uc *LogoutUseCase) Logout(ctx context.Context, in entity.LogoutInput) error {
	if in.OrgID == "" || in.RefreshToken == "" {
		return apperr.BadRequest("orgId and refreshToken are required")
	}
	hash := jwtutil.HashRefreshToken(in.RefreshToken)
	token, err := uc.refreshRepo.GetByHash(ctx, in.OrgID, hash)
	if err != nil {
		return nil
	}
	return uc.refreshRepo.Revoke(ctx, in.OrgID, token.ID, nil)
}
