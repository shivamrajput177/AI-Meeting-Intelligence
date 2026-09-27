package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

func TestReindexUseCase_Reindex_PartialFailureDoesNotAbortTheBatch(t *testing.T) {
	meetingsClient := &fakeMeetingsClient{meetings: []entity.Meeting{
		{ID: "meeting-1", Title: "Standup"},
		{ID: "meeting-2", Title: "Retro"},
	}}
	// meeting-1 has chunks to embed; meeting-2 doesn't (fakeChunksClient's
	// map has no entry for it, so ListChunks returns an empty slice, not
	// an error) — exercising the "some meetings genuinely have zero
	// chunks" case separately from the fetch-error case below.
	chunksClient := &fakeChunksClient{chunks: map[string][]entity.Chunk{
		"meeting-1": {{ID: "chunk-1", MeetingID: "meeting-1", Text: "hello"}},
	}}
	embedder := &fakeEmbedder{vec: []float32{0.1}}
	repo := &fakeRepository{}
	publisher := &fakePublisher{}
	log := logger.New("test", logger.LevelError)

	embedChunks := usecase.NewEmbedChunksUseCase(chunksClient, embedder, repo, publisher, log, "nomic-embed-text")
	uc := usecase.NewReindexUseCase(meetingsClient, embedChunks, log)

	result, err := uc.Reindex(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("Reindex: %v", err)
	}
	if result.MeetingsReindexed != 2 {
		t.Fatalf("expected both meetings to succeed (meeting-2 just has zero chunks, not an error), got %+v", result)
	}
	if len(result.FailedMeetingIDs) != 0 {
		t.Fatalf("expected no failures, got %+v", result.FailedMeetingIDs)
	}
}

func TestReindexUseCase_Reindex_EmbeddingErrorIsCountedAsFailedNotFatal(t *testing.T) {
	meetingsClient := &fakeMeetingsClient{meetings: []entity.Meeting{
		{ID: "meeting-1", Title: "Standup"},
		{ID: "meeting-2", Title: "Retro"},
	}}
	chunksClient := &fakeChunksClient{
		chunks: map[string][]entity.Chunk{"meeting-1": {{ID: "chunk-1", Text: "hello"}}, "meeting-2": {{ID: "chunk-2", Text: "hi"}}},
	}
	log := logger.New("test", logger.LevelError)
	repo := &fakeRepository{}
	publisher := &fakePublisher{}

	// The embedder fails outright, so every meeting's embed attempt
	// fails — Reindex itself must still return successfully (not
	// propagate the error), reporting every meeting as failed rather
	// than aborting partway through.
	embedChunks := usecase.NewEmbedChunksUseCase(chunksClient, &fakeEmbedder{err: errors.New("ollama down")}, repo, publisher, log, "nomic-embed-text")
	uc := usecase.NewReindexUseCase(meetingsClient, embedChunks, log)

	result, err := uc.Reindex(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("Reindex should not fail the whole batch over per-meeting errors: %v", err)
	}
	if result.MeetingsReindexed != 0 || len(result.FailedMeetingIDs) != 2 {
		t.Fatalf("expected both meetings to be reported failed, got %+v", result)
	}
}
