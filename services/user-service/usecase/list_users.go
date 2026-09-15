package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/repository"
)

type ListUsersUseCase struct {
	repo repository.Repository
}

func NewListUsersUseCase(repo repository.Repository) *ListUsersUseCase {
	return &ListUsersUseCase{repo: repo}
}

func (uc *ListUsersUseCase) ListUsers(ctx context.Context, orgID string, filter entity.ListUsersFilter) ([]*entity.User, int, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	return uc.repo.List(ctx, orgID, filter)
}
