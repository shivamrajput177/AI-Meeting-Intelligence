package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/repository"
)

type GetHistoryUseCase struct {
	repo repository.Repository
}

func NewGetHistoryUseCase(repo repository.Repository) *GetHistoryUseCase {
	return &GetHistoryUseCase{repo: repo}
}

// GetHistory is GET /qa/history's business logic — every past question
// this user has asked in this org, most recent first. Returns the raw
// cited_chunk_ids rather than fully re-resolved citations (meeting
// title/timestamp per chunk): re-resolving would mean a chunk_embeddings
// lookup plus a meetings.Client call per historical citation, on every
// history page load, for what's mostly a "did I ask this before"
// glance-back — a reasonable v1 scope line, not a technical limitation.
func (uc *GetHistoryUseCase) GetHistory(ctx context.Context, orgID, userID string) ([]*entity.QAHistoryEntry, error) {
	return uc.repo.ListQAHistory(ctx, orgID, userID)
}
