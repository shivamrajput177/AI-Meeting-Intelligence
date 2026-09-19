package usecase_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// fakeRepository, fakeStorage, fakeTranscriber, and fakePublisher are
// in-memory stand-ins for the real Postgres/MinIO/whisper.cpp/Kafka
// implementations — this is the whole point of defining each as an
// interface usecase depends on (see
// docs/architecture/folder-structure.md's Clean Architecture layering):
// ProcessUploadUseCase is fully testable with no real Postgres, MinIO,
// whisper.cpp, or Kafka involved.
type fakeRepository struct {
	mu         sync.Mutex
	transcript *entity.Transcript
	segments   []*entity.Segment
}

func (f *fakeRepository) CreateTranscript(_ context.Context, t *entity.Transcript) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.transcript = t
	return nil
}

func (f *fakeRepository) CreateSegments(_ context.Context, _ string, segments []*entity.Segment) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.segments = segments
	return nil
}

func (f *fakeRepository) GetByMeetingID(_ context.Context, _, _ string) (*entity.Transcript, []*entity.Segment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.transcript, f.segments, nil
}

type fakeStorage struct {
	object string
	err    error
}

func (f *fakeStorage) GetObject(_ context.Context, _ string) (io.ReadCloser, error) {
	if f.err != nil {
		return nil, f.err
	}
	return io.NopCloser(strings.NewReader(f.object)), nil
}

type fakeTranscriber struct {
	result *entity.TranscriptionResult
	err    error
}

func (f *fakeTranscriber) Transcribe(_ context.Context, _ io.Reader, _ string) (*entity.TranscriptionResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

type fakePublisher struct {
	mu        sync.Mutex
	completed []entity.TranscriptionCompletedEvent
	failed    []entity.TranscriptionFailedEvent
}

func (f *fakePublisher) PublishTranscriptionCompleted(_ context.Context, event entity.TranscriptionCompletedEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.completed = append(f.completed, event)
	return nil
}

func (f *fakePublisher) PublishTranscriptionFailed(_ context.Context, event entity.TranscriptionFailedEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failed = append(f.failed, event)
	return nil
}

func TestProcessUploadUseCase_ProcessUpload(t *testing.T) {
	repo := &fakeRepository{}
	publisher := &fakePublisher{}
	transcriber := &fakeTranscriber{result: &entity.TranscriptionResult{
		Language: "en",
		Text:     "hello world this is a test",
		Segments: []entity.TranscribedSegment{
			{StartMS: 0, EndMS: 1000, Text: "hello world"},
			{StartMS: 1000, EndMS: 2500, Text: "this is a test"},
		},
	}}
	uc := usecase.NewProcessUploadUseCase(repo, &fakeStorage{object: "fake audio bytes"}, transcriber, publisher, logger.New("test", logger.LevelError))

	transcript, err := uc.ProcessUpload(context.Background(), entity.MeetingUploadedEvent{
		MeetingID: "meeting-1", OrgID: "org-1", RecordingObjectKey: "org-1/meeting-1/rec.wav",
	})
	if err != nil {
		t.Fatalf("ProcessUpload: %v", err)
	}
	if transcript.Status != entity.StatusCompleted {
		t.Fatalf("Status = %q, want %q", transcript.Status, entity.StatusCompleted)
	}
	if transcript.WordCount != 6 {
		t.Fatalf("WordCount = %d, want 6", transcript.WordCount)
	}
	if len(repo.segments) != 2 {
		t.Fatalf("stored %d segments, want 2", len(repo.segments))
	}
	if repo.segments[0].SpeakerLabel != "speaker_1" {
		t.Fatalf("SpeakerLabel = %q, want single-speaker default", repo.segments[0].SpeakerLabel)
	}
	if len(publisher.completed) != 1 || publisher.completed[0].TranscriptID != transcript.ID {
		t.Fatalf("expected one transcription.completed.v1 event for transcript %q, got %+v", transcript.ID, publisher.completed)
	}
	if len(publisher.failed) != 0 {
		t.Fatalf("expected no failure events on success, got %+v", publisher.failed)
	}
}

func TestProcessUploadUseCase_ProcessUpload_StorageFailure(t *testing.T) {
	publisher := &fakePublisher{}
	uc := usecase.NewProcessUploadUseCase(&fakeRepository{}, &fakeStorage{err: errors.New("object not found")}, &fakeTranscriber{}, publisher, logger.New("test", logger.LevelError))

	if _, err := uc.ProcessUpload(context.Background(), entity.MeetingUploadedEvent{MeetingID: "meeting-1", OrgID: "org-1"}); err == nil {
		t.Fatal("expected an error when the recording can't be fetched")
	}
	// ProcessUpload itself doesn't publish the failure event — see its own
	// doc comment: that's consumer.go's retry loop's job, on exhaustion.
	if len(publisher.failed) != 0 {
		t.Fatalf("expected ProcessUpload not to publish transcription.failed.v1 itself, got %+v", publisher.failed)
	}
}

func TestProcessUploadUseCase_ProcessUpload_TranscriptionFailure(t *testing.T) {
	uc := usecase.NewProcessUploadUseCase(&fakeRepository{}, &fakeStorage{object: "audio"}, &fakeTranscriber{err: errors.New("whisper.cpp unreachable")}, &fakePublisher{}, logger.New("test", logger.LevelError))

	if _, err := uc.ProcessUpload(context.Background(), entity.MeetingUploadedEvent{MeetingID: "meeting-1", OrgID: "org-1"}); err == nil {
		t.Fatal("expected an error when transcription fails")
	}
}
