package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/authsvc/domain"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/jwtutil"
)

type LogoutInput struct {
	OrgID        string
	RefreshToken string
}

type LogoutUseCase struct {
	refreshRepo domain.RefreshTokenRepository
}

func NewLogoutUseCase(refreshRepo domain.RefreshTokenRepository) *LogoutUseCase {
	return &LogoutUseCase{refreshRepo: refreshRepo}
}

// Execute revokes the presented refresh token. It intentionally succeeds
// even if the token is already gone/invalid — logout is idempotent from
// the client's point of view ("am I logged out now?" — yes, either way).
func (uc *LogoutUseCase) Execute(ctx context.Context, in LogoutInput) error {
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
