package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/meetingsvc/domain"
)

type GetMeetingUseCase struct{ repo domain.Repository }

func NewGetMeetingUseCase(repo domain.Repository) *GetMeetingUseCase { return &GetMeetingUseCase{repo} }

func (uc *GetMeetingUseCase) Execute(ctx context.Context, orgID, id string) (*domain.Meeting, error) {
	return uc.repo.GetByID(ctx, orgID, id)
}

type ListMeetingsUseCase struct{ repo domain.Repository }

func NewListMeetingsUseCase(repo domain.Repository) *ListMeetingsUseCase {
	return &ListMeetingsUseCase{repo}
}

func (uc *ListMeetingsUseCase) Execute(ctx context.Context, orgID string, filter domain.ListFilter) ([]*domain.Meeting, int, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	return uc.repo.List(ctx, orgID, filter)
}
