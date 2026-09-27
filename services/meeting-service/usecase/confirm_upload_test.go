package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/kafkax"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/reqctx"
)

// fakeStorage is an in-memory stand-in for the real MinIO client.
type fakeStorage struct{ statErr error }

func (f *fakeStorage) PresignedPutURL(context.Context, string) (string, error) { return "", nil }
func (f *fakeStorage) Stat(context.Context, string) error                      { return f.statErr }
func (f *fakeStorage) Delete(context.Context, string) error                    { return nil }

func TestConfirmUploadUseCase_ConfirmUpload_StartsATrace(t *testing.T) {
	repo := &fakeRepository{meeting: &entity.Meeting{
		ID: "meeting-1", OrgID: "org-1", Title: "Standup", SourceType: "upload",
		RecordingObjectKey: "org-1/meeting-1/rec.webm", Status: entity.StatusUploaded,
	}}
	publisher := &fakePublisher{}
	uc := usecase.NewConfirmUploadUseCase(repo, &fakeStorage{}, publisher, logger.New("test", logger.LevelError))

	if _, err := uc.ConfirmUpload(context.Background(), "org-1", "meeting-1"); err != nil {
		t.Fatalf("ConfirmUpload: %v", err)
	}

	if len(publisher.uploaded) != 1 {
		t.Fatalf("expected one meeting.uploaded.v1 event, got %d", len(publisher.uploaded))
	}
	// See shared/kafkax's doc comment on HeaderTraceparent: ConfirmUpload
	// is where a trace begins, so the context it publishes with must
	// already carry a well-formed traceparent for kafkax.Publish to
	// attach as a Kafka header.
	tp := reqctx.Traceparent(publisher.uploadedCtx[0])
	if kafkax.TraceIDOf(tp) == "" {
		t.Fatalf("expected the publish context to carry a well-formed traceparent, got %q", tp)
	}
}

func TestConfirmUploadUseCase_ConfirmUpload_RecordingNotFound(t *testing.T) {
	repo := &fakeRepository{meeting: &entity.Meeting{ID: "meeting-1", OrgID: "org-1", RecordingObjectKey: "org-1/meeting-1/rec.webm"}}
	publisher := &fakePublisher{}
	uc := usecase.NewConfirmUploadUseCase(repo, &fakeStorage{statErr: errors.New("not found")}, publisher, logger.New("test", logger.LevelError))

	if _, err := uc.ConfirmUpload(context.Background(), "org-1", "meeting-1"); err == nil {
		t.Fatal("expected an error when the recording isn't in MinIO yet")
	}
	if len(publisher.uploaded) != 0 {
		t.Fatalf("expected no meeting.uploaded.v1 event when the upload can't be confirmed, got %+v", publisher.uploaded)
	}
}
