package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/repository"
)

type GetChunksUseCase struct {
	repo repository.Repository
}

func NewGetChunksUseCase(repo repository.Repository) *GetChunksUseCase {
	return &GetChunksUseCase{repo: repo}
}

func (uc *GetChunksUseCase) GetChunks(ctx context.Context, orgID, meetingID string) ([]*entity.Chunk, error) {
	return uc.repo.GetChunksByMeetingID(ctx, orgID, meetingID)
}
