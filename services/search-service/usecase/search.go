package usecase

import (
	"context"
	"fmt"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/llm"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/meetings"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/repository"
)

const defaultSearchLimit = 10

type SearchUseCase struct {
	embedder       llm.Embedder
	repo           repository.Repository
	meetingsClient meetings.Client
}

func NewSearchUseCase(embedder llm.Embedder, repo repository.Repository, meetingsClient meetings.Client) *SearchUseCase {
	return &SearchUseCase{embedder: embedder, repo: repo, meetingsClient: meetingsClient}
}

// SearchResult is what Search returns — a hit plus its resolved meeting
// title, so handler.go doesn't need its own second pass over
// meetingsClient.
type SearchResult struct {
	Hit          *entity.SearchHit
	MeetingTitle string
}

// Search is GET /search?q='s business logic — see
// docs/architecture/microservices.md §9's RAG flow, whose retrieval step
// this same query implements (AskUseCase calls it directly rather than
// duplicating the embed-then-retrieve logic).
func (uc *SearchUseCase) Search(ctx context.Context, orgID, query string) ([]SearchResult, error) {
	vec, err := uc.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	hits, err := uc.repo.SemanticSearch(ctx, orgID, vec, defaultSearchLimit)
	if err != nil {
		return nil, fmt.Errorf("semantic search: %w", err)
	}

	meetingIDs := make([]string, len(hits))
	for i, h := range hits {
		meetingIDs[i] = h.MeetingID
	}
	titles := resolveMeetingTitles(ctx, uc.meetingsClient, orgID, meetingIDs)

	results := make([]SearchResult, len(hits))
	for i, h := range hits {
		results[i] = SearchResult{Hit: h, MeetingTitle: titles[h.MeetingID]}
	}
	return results, nil
}
