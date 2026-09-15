package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/repository"
)

type GetMeetingUseCase struct{ repo repository.Repository }

func NewGetMeetingUseCase(repo repository.Repository) *GetMeetingUseCase {
	return &GetMeetingUseCase{repo}
}

func (uc *GetMeetingUseCase) GetMeeting(ctx context.Context, orgID, id string) (*entity.Meeting, error) {
	return uc.repo.GetByID(ctx, orgID, id)
}

type ListMeetingsUseCase struct{ repo repository.Repository }

func NewListMeetingsUseCase(repo repository.Repository) *ListMeetingsUseCase {
	return &ListMeetingsUseCase{repo}
}

func (uc *ListMeetingsUseCase) ListMeetings(ctx context.Context, orgID string, filter entity.ListFilter) ([]*entity.Meeting, int, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	return uc.repo.List(ctx, orgID, filter)
}
