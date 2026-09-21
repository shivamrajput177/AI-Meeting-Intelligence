package usecase

import (
	"strings"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/entity"
)

func words(n int) string {
	w := make([]string, n)
	for i := range w {
		w[i] = "word"
	}
	return strings.Join(w, " ")
}

func TestChunkSegments_Empty(t *testing.T) {
	if chunks := chunkSegments(nil); chunks != nil {
		t.Fatalf("expected nil chunks for no segments, got %+v", chunks)
	}
}

func TestChunkSegments_SingleChunkWhenShort(t *testing.T) {
	segments := []entity.TranscriptSegment{
		{StartMS: 0, EndMS: 1000, Text: words(50)},
		{StartMS: 1000, EndMS: 2000, Text: words(50)},
	}
	chunks := chunkSegments(segments)
	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1", len(chunks))
	}
	if chunks[0].StartMS != 0 || chunks[0].EndMS != 2000 {
		t.Fatalf("chunk span = [%d,%d], want [0,2000]", chunks[0].StartMS, chunks[0].EndMS)
	}
	if chunks[0].TokenCount != 100 {
		t.Fatalf("TokenCount = %d, want 100", chunks[0].TokenCount)
	}
}

func TestChunkSegments_SplitsAtTargetWithoutBreakingASegment(t *testing.T) {
	// Three segments of 300 words each: the first two (600 words) already
	// exceed targetChunkWords (500), so the split must happen at a
	// segment boundary, never mid-segment.
	segments := []entity.TranscriptSegment{
		{StartMS: 0, EndMS: 1000, Text: words(300)},
		{StartMS: 1000, EndMS: 2000, Text: words(300)},
		{StartMS: 2000, EndMS: 3000, Text: words(300)},
	}
	chunks := chunkSegments(segments)
	if len(chunks) < 2 {
		t.Fatalf("got %d chunks, want at least 2 for 900 words total", len(chunks))
	}
	// No chunk should ever contain a fragment of a segment's word count
	// that isn't a multiple of 300 (the fixed segment size here) plus
	// whatever whole overlap segments were carried in.
	for _, c := range chunks {
		if c.TokenCount%300 != 0 {
			t.Fatalf("chunk token count %d isn't a whole number of 300-word segments — a segment was split", c.TokenCount)
		}
	}
}

func TestChunkSegments_CarriesOverlapIntoNextChunk(t *testing.T) {
	// Two 600-word segments: the first alone is already under the 500-word
	// target (checked before it's added), but adding the second pushes the
	// running total over, forcing a flush at the seg1/seg2 boundary.
	segments := []entity.TranscriptSegment{
		{StartMS: 0, EndMS: 1000, Text: words(600)},
		{StartMS: 1000, EndMS: 2000, Text: words(600)},
	}
	chunks := chunkSegments(segments)
	if len(chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(chunks))
	}
	if chunks[0].StartMS != 0 || chunks[0].EndMS != 1000 || chunks[0].TokenCount != 600 {
		t.Fatalf("first chunk = span[%d,%d] tokens=%d, want span[0,1000] tokens=600",
			chunks[0].StartMS, chunks[0].EndMS, chunks[0].TokenCount)
	}
	// The second chunk's span starts back at 0, not 1000: overlap works in
	// whole-segment units (never splitting one), and the single preceding
	// segment (600 words) alone already exceeds overlapWords (50), so the
	// entire first segment is carried forward as overlap — a real,
	// accepted trade-off of "never split a segment" over hitting an exact
	// word-count overlap target.
	if chunks[1].StartMS != 0 || chunks[1].EndMS != 2000 || chunks[1].TokenCount != 1200 {
		t.Fatalf("second chunk = span[%d,%d] tokens=%d, want span[0,2000] tokens=1200 (overlap re-includes the whole preceding segment)",
			chunks[1].StartMS, chunks[1].EndMS, chunks[1].TokenCount)
	}
}
