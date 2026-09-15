package usecase

import (
	"context"
	"strings"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/domain"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
)

type UpdateProfileUseCase struct {
	repo domain.Repository
}

func NewUpdateProfileUseCase(repo domain.Repository) *UpdateProfileUseCase {
	return &UpdateProfileUseCase{repo: repo}
}

func (uc *UpdateProfileUseCase) UpdateProfile(ctx context.Context, in entity.UpdateProfileInput) (*domain.User, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, apperr.BadRequest("name is required")
	}
	return uc.repo.UpdateProfile(ctx, in.OrgID, in.UserID, name, in.AvatarURL)
}
