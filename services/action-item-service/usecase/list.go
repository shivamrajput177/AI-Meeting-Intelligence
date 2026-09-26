package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/repository"
)

type ListActionItemsByMeetingUseCase struct{ repo repository.Repository }

func NewListActionItemsByMeetingUseCase(repo repository.Repository) *ListActionItemsByMeetingUseCase {
	return &ListActionItemsByMeetingUseCase{repo}
}

func (uc *ListActionItemsByMeetingUseCase) ListByMeeting(ctx context.Context, orgID, meetingID string) ([]*entity.ActionItem, error) {
	return uc.repo.ListByMeeting(ctx, orgID, meetingID)
}

type ListActionItemsUseCase struct{ repo repository.Repository }

func NewListActionItemsUseCase(repo repository.Repository) *ListActionItemsUseCase {
	return &ListActionItemsUseCase{repo}
}

func (uc *ListActionItemsUseCase) List(ctx context.Context, orgID string, filter entity.ListActionItemsFilter) ([]*entity.ActionItem, int, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	return uc.repo.List(ctx, orgID, filter)
}
