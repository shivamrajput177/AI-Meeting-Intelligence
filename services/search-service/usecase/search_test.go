package usecase_test

import (
	"context"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/usecase"
)

func TestSearchUseCase_Search(t *testing.T) {
	embedder := &fakeEmbedder{vec: []float32{0.1, 0.2}}
	repo := &fakeRepository{searchHits: []*entity.SearchHit{
		{ChunkID: "chunk-1", MeetingID: "meeting-1", Text: "we shipped on friday", StartMS: 5000, Score: 0.1},
	}}
	meetingsClient := &fakeMeetingsClient{titles: map[string]string{"meeting-1": "Standup"}}

	uc := usecase.NewSearchUseCase(embedder, repo, meetingsClient)
	results, err := uc.Search(context.Background(), "org-1", "when did we ship")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].MeetingTitle != "Standup" {
		t.Errorf("MeetingTitle = %q, want Standup", results[0].MeetingTitle)
	}
	if results[0].Hit.ChunkID != "chunk-1" {
		t.Errorf("ChunkID = %q, want chunk-1", results[0].Hit.ChunkID)
	}
}

func TestSimilarMeetingsUseCase_Similar(t *testing.T) {
	repo := &fakeRepository{
		embeddings: map[string][]*entity.ChunkEmbedding{
			"meeting-1": {{Embedding: []float32{1, 0}}, {Embedding: []float32{0, 1}}},
		},
		similar: []*entity.SimilarMeeting{{MeetingID: "meeting-2", Score: 0.2}},
	}
	meetingsClient := &fakeMeetingsClient{titles: map[string]string{"meeting-2": "Retro"}}

	uc := usecase.NewSimilarMeetingsUseCase(repo, meetingsClient)
	results, err := uc.Similar(context.Background(), "org-1", "meeting-1")
	if err != nil {
		t.Fatalf("Similar: %v", err)
	}
	if len(results) != 1 || results[0].Title != "Retro" {
		t.Fatalf("expected one result titled Retro, got %+v", results)
	}
}

func TestSimilarMeetingsUseCase_Similar_NoEmbeddingsYet(t *testing.T) {
	uc := usecase.NewSimilarMeetingsUseCase(&fakeRepository{}, &fakeMeetingsClient{})
	if _, err := uc.Similar(context.Background(), "org-1", "meeting-1"); err == nil {
		t.Fatal("expected an error when the meeting has no indexed chunks yet")
	}
}
