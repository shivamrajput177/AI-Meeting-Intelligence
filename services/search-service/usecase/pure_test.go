package usecase

import (
	"strings"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/entity"
)

func TestCentroidOf(t *testing.T) {
	embeddings := []*entity.ChunkEmbedding{
		{Embedding: []float32{1, 2, 3}},
		{Embedding: []float32{3, 4, 5}},
	}
	got := centroidOf(embeddings)
	want := []float32{2, 3, 4}
	if len(got) != len(want) {
		t.Fatalf("centroidOf length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("centroidOf[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestCentroidOf_Empty(t *testing.T) {
	if got := centroidOf(nil); got != nil {
		t.Errorf("centroidOf(nil) = %v, want nil", got)
	}
}

func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		ms   int
		want string
	}{
		{0, "0:00"},
		{5000, "0:05"},
		{65000, "1:05"},
		{600000, "10:00"},
	}
	for _, tt := range tests {
		if got := formatTimestamp(tt.ms); got != tt.want {
			t.Errorf("formatTimestamp(%d) = %q, want %q", tt.ms, got, tt.want)
		}
	}
}

func TestBuildContext(t *testing.T) {
	results := []SearchResult{
		{Hit: &entity.SearchHit{ChunkID: "chunk-1", MeetingID: "meeting-1", Text: "we shipped on friday", StartMS: 5000}, MeetingTitle: "Standup"},
	}
	groundedContext, citations, chunkIDs := buildContext(results)

	if !strings.Contains(groundedContext, "[Standup, 0:05]: we shipped on friday") {
		t.Errorf("groundedContext = %q, want it to contain a formatted citation header", groundedContext)
	}
	if len(citations) != 1 || citations[0].MeetingTitle != "Standup" {
		t.Fatalf("citations = %+v, want one citation for Standup", citations)
	}
	if len(chunkIDs) != 1 || chunkIDs[0] != "chunk-1" {
		t.Fatalf("chunkIDs = %v, want [chunk-1]", chunkIDs)
	}
}
