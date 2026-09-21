package usecase_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// fakeTranscriptClient, fakeSummarizer, fakeRepository, and
// fakePublisher are in-memory stand-ins for the real Transcription
// Service HTTP client / Ollama client / Postgres repository / Kafka
// producer — this is the whole point of defining each as an interface
// usecase depends on (see docs/architecture/folder-structure.md's Clean
// Architecture layering): ProcessTranscriptUseCase is fully testable
// with none of those real systems involved.
type fakeTranscriptClient struct {
	transcript *entity.Transcript
	err        error
}

func (f *fakeTranscriptClient) GetTranscript(_ context.Context, _, _ string) (*entity.Transcript, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.transcript, nil
}

type fakeSummarizer struct {
	result *entity.SummaryResult
	err    error
}

func (f *fakeSummarizer) Summarize(_ context.Context, _ string) (*entity.SummaryResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

type fakeRepository struct {
	mu      sync.Mutex
	summary *entity.Summary
	chunks  []*entity.Chunk
}

func (f *fakeRepository) UpsertSummary(_ context.Context, _ string, s *entity.Summary) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.summary = s
	return nil
}

func (f *fakeRepository) GetSummaryByMeetingID(_ context.Context, _, _ string) (*entity.Summary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.summary, nil
}

func (f *fakeRepository) ReplaceChunks(_ context.Context, _, _ string, chunks []*entity.Chunk) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.chunks = chunks
	return nil
}

type fakePublisher struct {
	mu               sync.Mutex
	chunkCreated     []entity.ChunkCreatedEvent
	summaryCompleted []entity.SummaryCompletedEvent
	summaryFailed    []entity.SummaryFailedEvent
}

func (f *fakePublisher) PublishChunkCreated(_ context.Context, event entity.ChunkCreatedEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.chunkCreated = append(f.chunkCreated, event)
	return nil
}

func (f *fakePublisher) PublishSummaryCompleted(_ context.Context, event entity.SummaryCompletedEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.summaryCompleted = append(f.summaryCompleted, event)
	return nil
}

func (f *fakePublisher) PublishSummaryFailed(_ context.Context, event entity.SummaryFailedEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.summaryFailed = append(f.summaryFailed, event)
	return nil
}

func TestProcessTranscriptUseCase_ProcessTranscript(t *testing.T) {
	transcriptClient := &fakeTranscriptClient{transcript: &entity.Transcript{
		MeetingID: "meeting-1",
		RawText:   "we decided to ship on friday. risk: the vendor api might be flaky. blocked on design review.",
		Segments: []entity.TranscriptSegment{
			{StartMS: 0, EndMS: 1000, Text: "we decided to ship on friday."},
			{StartMS: 1000, EndMS: 2000, Text: "risk: the vendor api might be flaky."},
			{StartMS: 2000, EndMS: 3000, Text: "blocked on design review."},
		},
	}}
	summarizer := &fakeSummarizer{result: &entity.SummaryResult{
		SummaryText:  "Team agreed to ship Friday; flagged a vendor API risk and a design-review blocker.",
		KeyDecisions: []string{"Ship on Friday"},
		Risks:        []string{"Vendor API might be flaky"},
		Blockers:     []string{"Design review"},
	}}
	repo := &fakeRepository{}
	publisher := &fakePublisher{}
	uc := usecase.NewProcessTranscriptUseCase(transcriptClient, summarizer, repo, publisher, logger.New("test", logger.LevelError), "qwen2.5:7b", "v1")

	summary, err := uc.ProcessTranscript(context.Background(), "org-1", "meeting-1")
	if err != nil {
		t.Fatalf("ProcessTranscript: %v", err)
	}
	if summary.SummaryText != summarizer.result.SummaryText {
		t.Fatalf("SummaryText = %q, want %q", summary.SummaryText, summarizer.result.SummaryText)
	}
	if summary.ModelUsed != "qwen2.5:7b" || summary.PromptVersion != "v1" {
		t.Fatalf("ModelUsed/PromptVersion = %q/%q, want qwen2.5:7b/v1", summary.ModelUsed, summary.PromptVersion)
	}
	if len(repo.chunks) == 0 {
		t.Fatal("expected chunks to be persisted")
	}
	if len(publisher.chunkCreated) != 1 || publisher.chunkCreated[0].ChunkCount != len(repo.chunks) {
		t.Fatalf("expected one chunk.created.v1 event matching stored chunk count, got %+v (stored %d)", publisher.chunkCreated, len(repo.chunks))
	}
	if len(publisher.summaryCompleted) != 1 || publisher.summaryCompleted[0].SummaryID != summary.ID {
		t.Fatalf("expected one summary.completed.v1 event for summary %q, got %+v", summary.ID, publisher.summaryCompleted)
	}
	if len(publisher.summaryFailed) != 0 {
		t.Fatalf("expected no failure events on success, got %+v", publisher.summaryFailed)
	}
}

func TestProcessTranscriptUseCase_ProcessTranscript_TranscriptFetchFailure(t *testing.T) {
	publisher := &fakePublisher{}
	uc := usecase.NewProcessTranscriptUseCase(
		&fakeTranscriptClient{err: errors.New("transcription service unreachable")},
		&fakeSummarizer{}, &fakeRepository{}, publisher, logger.New("test", logger.LevelError), "qwen2.5:7b", "v1",
	)

	if _, err := uc.ProcessTranscript(context.Background(), "org-1", "meeting-1"); err == nil {
		t.Fatal("expected an error when the transcript can't be fetched")
	}
	// ProcessTranscript itself doesn't publish the failure event — see its
	// own doc comment: that's consumer.go's retry loop's job, on
	// exhaustion.
	if len(publisher.summaryFailed) != 0 {
		t.Fatalf("expected ProcessTranscript not to publish summary.failed.v1 itself, got %+v", publisher.summaryFailed)
	}
}

func TestProcessTranscriptUseCase_ProcessTranscript_SummarizationFailure(t *testing.T) {
	transcriptClient := &fakeTranscriptClient{transcript: &entity.Transcript{
		MeetingID: "meeting-1",
		RawText:   "hello",
		Segments:  []entity.TranscriptSegment{{StartMS: 0, EndMS: 100, Text: "hello"}},
	}}
	uc := usecase.NewProcessTranscriptUseCase(
		transcriptClient, &fakeSummarizer{err: errors.New("ollama unreachable")},
		&fakeRepository{}, &fakePublisher{}, logger.New("test", logger.LevelError), "qwen2.5:7b", "v1",
	)

	if _, err := uc.ProcessTranscript(context.Background(), "org-1", "meeting-1"); err == nil {
		t.Fatal("expected an error when summarization fails")
	}
}
