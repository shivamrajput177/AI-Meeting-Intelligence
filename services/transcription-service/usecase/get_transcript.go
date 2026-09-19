package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/repository"
)

type GetTranscriptUseCase struct {
	repo repository.Repository
}

func NewGetTranscriptUseCase(repo repository.Repository) *GetTranscriptUseCase {
	return &GetTranscriptUseCase{repo: repo}
}

func (uc *GetTranscriptUseCase) GetTranscript(ctx context.Context, orgID, meetingID string) (*entity.Transcript, []*entity.Segment, error) {
	return uc.repo.GetByMeetingID(ctx, orgID, meetingID)
}
