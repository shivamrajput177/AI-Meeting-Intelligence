package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/events"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/llm"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/transcript"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

type ProcessTranscriptUseCase struct {
	transcriptClient transcript.Client
	summarizer       llm.Summarizer
	repo             repository.Repository
	publisher        events.Publisher
	log              *logger.Logger
	modelUsed        string
	promptVersion    string
}

func NewProcessTranscriptUseCase(
	transcriptClient transcript.Client,
	summarizer llm.Summarizer,
	repo repository.Repository,
	publisher events.Publisher,
	log *logger.Logger,
	modelUsed string,
	promptVersion string,
) *ProcessTranscriptUseCase {
	return &ProcessTranscriptUseCase{
		transcriptClient: transcriptClient, summarizer: summarizer, repo: repo,
		publisher: publisher, log: log, modelUsed: modelUsed, promptVersion: promptVersion,
	}
}

// ProcessTranscript is transcription.completed.v1's business logic (see
// consumer.go), and is also called directly by POST
// /meetings/{id}/summary/regenerate to re-run the exact same pipeline on
// demand: fetch the transcript from Transcription Service, chunk it,
// summarize it via Ollama, persist both, publish chunk.created.v1 and
// summary.completed.v1. It deliberately doesn't publish a failure event
// itself — same reasoning as transcription-service's
// ProcessUploadUseCase: consumer.go's retry loop owns deciding when an
// error is worth retrying versus when to give up and publish
// summary.failed.v1, so that decision lives in exactly one place.
func (uc *ProcessTranscriptUseCase) ProcessTranscript(ctx context.Context, orgID, meetingID string) (*entity.Summary, error) {
	t, err := uc.transcriptClient.GetTranscript(ctx, orgID, meetingID)
	if err != nil {
		return nil, fmt.Errorf("fetch transcript: %w", err)
	}

	chunks := chunkSegments(t.Segments)
	persistedChunks := make([]*entity.Chunk, len(chunks))
	for i := range chunks {
		c := chunks[i]
		c.ID = uuid.NewString()
		c.MeetingID = meetingID
		c.ChunkIndex = i
		persistedChunks[i] = &c
	}
	if err := uc.repo.ReplaceChunks(ctx, orgID, meetingID, persistedChunks); err != nil {
		return nil, fmt.Errorf("store chunks: %w", err)
	}

	result, err := uc.summarizer.Summarize(ctx, t.RawText)
	if err != nil {
		return nil, fmt.Errorf("ollama summarization: %w", err)
	}

	summary := &entity.Summary{
		ID: uuid.NewString(), MeetingID: meetingID, OrgID: orgID,
		SummaryText: result.SummaryText, KeyDecisions: result.KeyDecisions,
		Risks: result.Risks, Blockers: result.Blockers,
		ModelUsed: uc.modelUsed, PromptVersion: uc.promptVersion, CreatedAt: time.Now(),
	}
	if err := uc.repo.UpsertSummary(ctx, orgID, summary); err != nil {
		return nil, fmt.Errorf("store summary: %w", err)
	}

	if err := uc.publisher.PublishChunkCreated(ctx, entity.ChunkCreatedEvent{
		MeetingID: meetingID, OrgID: orgID, ChunkCount: len(persistedChunks),
	}); err != nil {
		// Best-effort, same trade-off as meeting-service's ConfirmUpload
		// and transcription-service's ProcessUploadUseCase: no
		// transactional outbox yet, so a dropped event here is a real,
		// logged gap rather than a silently swallowed one.
		uc.log.Error("publish chunk.created.v1 failed", "meeting_id", meetingID, "err", err)
	}
	if err := uc.publisher.PublishSummaryCompleted(ctx, entity.SummaryCompletedEvent{
		MeetingID: meetingID, OrgID: orgID, SummaryID: summary.ID,
	}); err != nil {
		uc.log.Error("publish summary.completed.v1 failed", "meeting_id", meetingID, "err", err)
	}
	return summary, nil
}
