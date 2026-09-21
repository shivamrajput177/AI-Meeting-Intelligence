package usecase

import (
	"strings"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/entity"
)

// targetChunkWords/overlapWords approximate the "~500-token overlapping
// windows" docs/architecture/microservices.md §7 describes — word count
// is used as a token-count proxy (no real tokenizer dependency for this
// walking-skeleton pass; a model-specific tokenizer is a fine later
// upgrade if chunk-size accuracy against a real context window ever
// matters more than it does today).
const (
	targetChunkWords = 500
	overlapWords     = 50
)

// chunkSegments splits transcript segments into speaker-aware windows —
// a single segment is never split across two chunks, since a segment is
// already one continuous span of speech — with a trailing overlap
// carried into the start of the next chunk so context isn't lost at a
// chunk boundary. Returned chunks have no ID/MeetingID set yet: the
// caller (ProcessTranscriptUseCase) fills those in before persisting.
func chunkSegments(segments []entity.TranscriptSegment) []entity.Chunk {
	if len(segments) == 0 {
		return nil
	}

	var chunks []entity.Chunk
	var current []entity.TranscriptSegment
	wordCount := 0

	flush := func() {
		if len(current) == 0 {
			return
		}
		chunks = append(chunks, buildChunk(current))
	}

	for _, seg := range segments {
		segWords := len(strings.Fields(seg.Text))
		if wordCount+segWords > targetChunkWords && len(current) > 0 {
			flush()
			current = overlapTail(current)
			wordCount = wordsIn(current)
		}
		current = append(current, seg)
		wordCount += segWords
	}
	flush()
	return chunks
}

func buildChunk(segments []entity.TranscriptSegment) entity.Chunk {
	var text strings.Builder
	for i, s := range segments {
		if i > 0 {
			text.WriteString(" ")
		}
		text.WriteString(s.Text)
	}
	return entity.Chunk{
		Text:       text.String(),
		TokenCount: len(strings.Fields(text.String())),
		StartMS:    segments[0].StartMS,
		EndMS:      segments[len(segments)-1].EndMS,
	}
}

// overlapTail returns the trailing segments of current totaling roughly
// overlapWords — in whole-segment units, so a single long segment (a
// common case: one person talking for a while) can alone overshoot
// overlapWords by a lot. That's an accepted trade-off of never splitting
// a segment (the whole point of "speaker-aware" chunking) over hitting
// an exact word-count overlap target.
func overlapTail(current []entity.TranscriptSegment) []entity.TranscriptSegment {
	words := 0
	start := len(current)
	for start > 0 && words < overlapWords {
		start--
		words += len(strings.Fields(current[start].Text))
	}
	tail := make([]entity.TranscriptSegment, len(current)-start)
	copy(tail, current[start:])
	return tail
}

func wordsIn(segments []entity.TranscriptSegment) int {
	n := 0
	for _, s := range segments {
		n += len(strings.Fields(s.Text))
	}
	return n
}
