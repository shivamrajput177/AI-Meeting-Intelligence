package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/repository"
)

type GetActionItemUseCase struct{ repo repository.Repository }

func NewGetActionItemUseCase(repo repository.Repository) *GetActionItemUseCase {
	return &GetActionItemUseCase{repo}
}

func (uc *GetActionItemUseCase) GetActionItem(ctx context.Context, orgID, id string) (*entity.ActionItem, error) {
	return uc.repo.GetByID(ctx, orgID, id)
}
