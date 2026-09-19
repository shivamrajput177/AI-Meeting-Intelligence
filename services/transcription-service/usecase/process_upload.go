package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/asr"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/events"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/storage"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// modelName is a dev-only fixed value — whisper.cpp loads one model per
// server process (see asr/whispercpp's doc comment), so this just records
// which one for the transcript row; making it configurable per-request
// isn't meaningful until a service can address multiple whisper.cpp
// servers running different models.
const modelName = "base"

type ProcessUploadUseCase struct {
	repo        repository.Repository
	storage     storage.ObjectStorage
	transcriber asr.Transcriber
	publisher   events.Publisher
	log         *logger.Logger
}

func NewProcessUploadUseCase(repo repository.Repository, storage storage.ObjectStorage, transcriber asr.Transcriber, publisher events.Publisher, log *logger.Logger) *ProcessUploadUseCase {
	return &ProcessUploadUseCase{repo: repo, storage: storage, transcriber: transcriber, publisher: publisher, log: log}
}

// ProcessUpload is meeting.uploaded.v1's business logic — fetch the
// recording, run it through whisper.cpp, persist the transcript +
// segments, publish transcription.completed.v1. It deliberately doesn't
// publish a failure event itself: consumer.go's retry loop owns deciding
// when an error is worth retrying versus when to give up and publish
// transcription.failed.v1, so that decision lives in exactly one place
// (see consumer.go's doc comment).
func (uc *ProcessUploadUseCase) ProcessUpload(ctx context.Context, in entity.MeetingUploadedEvent) (*entity.Transcript, error) {
	object, err := uc.storage.GetObject(ctx, in.RecordingObjectKey)
	if err != nil {
		return nil, fmt.Errorf("fetch recording: %w", err)
	}
	defer func() { _ = object.Close() }()

	result, err := uc.transcriber.Transcribe(ctx, object, in.RecordingObjectKey)
	if err != nil {
		return nil, fmt.Errorf("whisper.cpp transcription: %w", err)
	}

	transcript := &entity.Transcript{
		ID:        uuid.NewString(),
		MeetingID: in.MeetingID,
		OrgID:     in.OrgID,
		Language:  result.Language,
		Engine:    "whisper.cpp",
		ModelName: modelName,
		Status:    entity.StatusCompleted,
		RawText:   result.Text,
		WordCount: len(strings.Fields(result.Text)),
		CreatedAt: time.Now(),
	}
	if err := uc.repo.CreateTranscript(ctx, transcript); err != nil {
		return nil, fmt.Errorf("store transcript: %w", err)
	}

	segments := make([]*entity.Segment, 0, len(result.Segments))
	for _, s := range result.Segments {
		segments = append(segments, &entity.Segment{
			TranscriptID: transcript.ID,
			// Single-speaker label — no diarization model in Phase 2, see
			// docs/architecture/microservices.md §6 ("speaker segments if
			// diarization model available, else single-speaker").
			SpeakerLabel: "speaker_1",
			StartMS:      s.StartMS,
			EndMS:        s.EndMS,
			Text:         s.Text,
		})
	}
	if err := uc.repo.CreateSegments(ctx, in.OrgID, segments); err != nil {
		return nil, fmt.Errorf("store segments: %w", err)
	}

	if err := uc.publisher.PublishTranscriptionCompleted(ctx, entity.TranscriptionCompletedEvent{
		MeetingID: in.MeetingID, OrgID: in.OrgID, TranscriptID: transcript.ID,
	}); err != nil {
		// Best-effort, same trade-off as meeting-service's ConfirmUpload:
		// no transactional outbox yet, so a dropped event here is a real
		// gap (the meeting sits in "transcribed-but-nobody-knows" until a
		// future retry/reconciliation mechanism exists), logged loudly
		// rather than silently swallowed.
		uc.log.Error("publish transcription.completed.v1 failed", "meeting_id", in.MeetingID, "err", err)
	}
	return transcript, nil
}
