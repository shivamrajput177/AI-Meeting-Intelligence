package usecase

import (
	"context"
	"fmt"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/meetings"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
)

const defaultSimilarLimit = 5

type SimilarMeetingsUseCase struct {
	repo           repository.Repository
	meetingsClient meetings.Client
}

func NewSimilarMeetingsUseCase(repo repository.Repository, meetingsClient meetings.Client) *SimilarMeetingsUseCase {
	return &SimilarMeetingsUseCase{repo: repo, meetingsClient: meetingsClient}
}

type SimilarMeetingResult struct {
	Meeting *entity.SimilarMeeting
	Title   string
}

// Similar is GET /meetings/{id}/similar's business logic. A meeting has
// many chunk embeddings, not one, so "similar to this meeting" needs a
// single representative vector first — computed here as the plain
// componentwise average (centroid) of all its chunk embeddings, in Go,
// rather than a SQL `avg(vector)` aggregate: that aggregate only exists
// in pgvector >= 0.5, and this project has never run against a real
// Postgres in this sandbox to confirm which version the
// pgvector/pgvector:pg16 image bundles (see README's Status section) —
// computing it in Go needs nothing from the extension beyond the `<=>`
// distance operator every pgvector version has always had.
func (uc *SimilarMeetingsUseCase) Similar(ctx context.Context, orgID, meetingID string) ([]SimilarMeetingResult, error) {
	embeddings, err := uc.repo.GetEmbeddingsByMeeting(ctx, orgID, meetingID)
	if err != nil {
		return nil, fmt.Errorf("fetch meeting embeddings: %w", err)
	}
	if len(embeddings) == 0 {
		return nil, apperr.NotFound("meeting has no indexed chunks yet")
	}

	centroid := centroidOf(embeddings)
	matches, err := uc.repo.SimilarMeetings(ctx, orgID, meetingID, centroid, defaultSimilarLimit)
	if err != nil {
		return nil, fmt.Errorf("similar meetings query: %w", err)
	}

	meetingIDs := make([]string, len(matches))
	for i, m := range matches {
		meetingIDs[i] = m.MeetingID
	}
	titles := resolveMeetingTitles(ctx, uc.meetingsClient, orgID, meetingIDs)

	results := make([]SimilarMeetingResult, len(matches))
	for i, m := range matches {
		results[i] = SimilarMeetingResult{Meeting: m, Title: titles[m.MeetingID]}
	}
	return results, nil
}

// centroidOf averages a set of equal-length embeddings componentwise — a
// pure function, easy to unit test without any repository/Ollama
// involved.
func centroidOf(embeddings []*entity.ChunkEmbedding) []float32 {
	if len(embeddings) == 0 {
		return nil
	}
	dims := len(embeddings[0].Embedding)
	sum := make([]float64, dims)
	for _, e := range embeddings {
		for i, v := range e.Embedding {
			sum[i] += float64(v)
		}
	}
	centroid := make([]float32, dims)
	for i, s := range sum {
		centroid[i] = float32(s / float64(len(embeddings)))
	}
	return centroid
}
