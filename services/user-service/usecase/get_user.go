package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/repository"
)

type GetUserUseCase struct {
	repo repository.Repository
}

func NewGetUserUseCase(repo repository.Repository) *GetUserUseCase {
	return &GetUserUseCase{repo: repo}
}

func (uc *GetUserUseCase) GetUser(ctx context.Context, orgID, userID string) (*entity.User, error) {
	return uc.repo.GetByID(ctx, orgID, userID)
}
