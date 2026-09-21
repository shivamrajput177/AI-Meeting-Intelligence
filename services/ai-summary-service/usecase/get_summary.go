package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/repository"
)

type GetSummaryUseCase struct {
	repo repository.Repository
}

func NewGetSummaryUseCase(repo repository.Repository) *GetSummaryUseCase {
	return &GetSummaryUseCase{repo: repo}
}

func (uc *GetSummaryUseCase) GetSummary(ctx context.Context, orgID, meetingID string) (*entity.Summary, error) {
	return uc.repo.GetSummaryByMeetingID(ctx, orgID, meetingID)
}
