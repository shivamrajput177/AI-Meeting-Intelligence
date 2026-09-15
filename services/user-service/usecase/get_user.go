package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/domain"
)

type GetUserUseCase struct {
	repo domain.Repository
}

func NewGetUserUseCase(repo domain.Repository) *GetUserUseCase {
	return &GetUserUseCase{repo: repo}
}

func (uc *GetUserUseCase) GetUser(ctx context.Context, orgID, userID string) (*domain.User, error) {
	return uc.repo.GetByID(ctx, orgID, userID)
}
