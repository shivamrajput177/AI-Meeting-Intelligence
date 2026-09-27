package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

func TestEmbedChunksUseCase_EmbedChunks(t *testing.T) {
	chunksClient := &fakeChunksClient{chunks: map[string][]entity.Chunk{
		"meeting-1": {
			{ID: "chunk-1", MeetingID: "meeting-1", Text: "we shipped on friday", StartMS: 0, EndMS: 1000},
			{ID: "chunk-2", MeetingID: "meeting-1", Text: "risk: vendor api flaky", StartMS: 1000, EndMS: 2000},
		},
	}}
	embedder := &fakeEmbedder{vec: []float32{0.1, 0.2, 0.3}}
	repo := &fakeRepository{}
	publisher := &fakePublisher{}

	uc := usecase.NewEmbedChunksUseCase(chunksClient, embedder, repo, publisher, logger.New("test", logger.LevelError), "nomic-embed-text")
	embeddings, err := uc.EmbedChunks(context.Background(), "org-1", "meeting-1")
	if err != nil {
		t.Fatalf("EmbedChunks: %v", err)
	}
	if len(embeddings) != 2 {
		t.Fatalf("expected 2 embeddings, got %d", len(embeddings))
	}
	if embeddings[0].ModelName != "nomic-embed-text" {
		t.Errorf("ModelName = %q, want nomic-embed-text", embeddings[0].ModelName)
	}
	if len(repo.embeddings["meeting-1"]) != 2 {
		t.Fatalf("expected 2 embeddings persisted, got %d", len(repo.embeddings["meeting-1"]))
	}
	if len(publisher.completed) != 1 || publisher.completed[0].ChunkCount != 2 {
		t.Fatalf("expected one embedding.completed.v1 event with count 2, got %+v", publisher.completed)
	}
	if len(publisher.failed) != 0 {
		t.Fatalf("expected no failure events on success, got %+v", publisher.failed)
	}
}

func TestEmbedChunksUseCase_EmbedChunks_ChunksFetchFailure(t *testing.T) {
	publisher := &fakePublisher{}
	uc := usecase.NewEmbedChunksUseCase(
		&fakeChunksClient{err: errors.New("ai-summary-service unreachable")},
		&fakeEmbedder{}, &fakeRepository{}, publisher, logger.New("test", logger.LevelError), "nomic-embed-text",
	)

	if _, err := uc.EmbedChunks(context.Background(), "org-1", "meeting-1"); err == nil {
		t.Fatal("expected an error when chunks can't be fetched")
	}
	// EmbedChunks itself doesn't publish the failure event — see its own
	// doc comment: that's consumer.go's retry loop's job, on exhaustion.
	if len(publisher.failed) != 0 {
		t.Fatalf("expected EmbedChunks not to publish embedding.failed.v1 itself, got %+v", publisher.failed)
	}
}

func TestEmbedChunksUseCase_EmbedChunks_EmbeddingFailure(t *testing.T) {
	chunksClient := &fakeChunksClient{chunks: map[string][]entity.Chunk{
		"meeting-1": {{ID: "chunk-1", MeetingID: "meeting-1", Text: "hello"}},
	}}
	uc := usecase.NewEmbedChunksUseCase(
		chunksClient, &fakeEmbedder{err: errors.New("ollama unreachable")},
		&fakeRepository{}, &fakePublisher{}, logger.New("test", logger.LevelError), "nomic-embed-text",
	)

	if _, err := uc.EmbedChunks(context.Background(), "org-1", "meeting-1"); err == nil {
		t.Fatal("expected an error when embedding fails")
	}
}
