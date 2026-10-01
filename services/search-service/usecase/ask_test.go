package usecase_test

import (
	"context"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/usecase"
)

func TestAskUseCase_Ask(t *testing.T) {
	embedder := &fakeEmbedder{vec: []float32{0.1}}
	repo := &fakeRepository{searchHits: []*entity.SearchHit{
		{ChunkID: "chunk-1", MeetingID: "meeting-1", Text: "we shipped on friday", StartMS: 5000},
	}}
	meetingsClient := &fakeMeetingsClient{titles: map[string]string{"meeting-1": "Standup"}}
	answerer := &fakeAnswerer{answer: "The team shipped on Friday."}

	search := usecase.NewSearchUseCase(embedder, repo, meetingsClient)
	uc := usecase.NewAskUseCase(search, answerer, repo, "qwen2.5:7b")

	result, err := uc.Ask(context.Background(), "org-1", "user-1", "when did we ship")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if result.Answer != "The team shipped on Friday." {
		t.Errorf("Answer = %q, want %q", result.Answer, "The team shipped on Friday.")
	}
	if len(result.Citations) != 1 || result.Citations[0].MeetingTitle != "Standup" {
		t.Fatalf("expected one citation for Standup, got %+v", result.Citations)
	}
	if len(repo.qaHistory) != 1 || repo.qaHistory[0].Answer != result.Answer {
		t.Fatalf("expected the Q&A to be persisted to history, got %+v", repo.qaHistory)
	}
	if len(repo.qaHistory[0].CitedChunkIDs) != 1 || repo.qaHistory[0].CitedChunkIDs[0] != "chunk-1" {
		t.Fatalf("expected cited_chunk_ids = [chunk-1], got %v", repo.qaHistory[0].CitedChunkIDs)
	}
}
