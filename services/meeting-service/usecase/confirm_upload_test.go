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
type fakeStorage struct {
	statErr  error
	size     int64
	deleted  []string
	deleteFn func(string) error
}

func (f *fakeStorage) PresignedPutURL(context.Context, string) (string, error) { return "", nil }
func (f *fakeStorage) Stat(context.Context, string) (int64, error)             { return f.size, f.statErr }
func (f *fakeStorage) Delete(_ context.Context, objectKey string) error {
	f.deleted = append(f.deleted, objectKey)
	if f.deleteFn != nil {
		return f.deleteFn(objectKey)
	}
	return nil
}

func TestConfirmUploadUseCase_ConfirmUpload_StartsATrace(t *testing.T) {
	repo := &fakeRepository{meeting: &entity.Meeting{
		ID: "meeting-1", OrgID: "org-1", Title: "Standup", SourceType: "upload",
		RecordingObjectKey: "org-1/meeting-1/rec.webm", Status: entity.StatusUploaded,
	}}
	publisher := &fakePublisher{}
	uc := usecase.NewConfirmUploadUseCase(repo, &fakeStorage{}, publisher, logger.New("test", logger.LevelError), 0)

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
	uc := usecase.NewConfirmUploadUseCase(repo, &fakeStorage{statErr: errors.New("not found")}, publisher, logger.New("test", logger.LevelError), 0)

	if _, err := uc.ConfirmUpload(context.Background(), "org-1", "meeting-1"); err == nil {
		t.Fatal("expected an error when the recording isn't in MinIO yet")
	}
	if len(publisher.uploaded) != 0 {
		t.Fatalf("expected no meeting.uploaded.v1 event when the upload can't be confirmed, got %+v", publisher.uploaded)
	}
}

// Phase 7's demo-mode clip-length guard (see ConfirmUploadUseCase's own doc
// comment on maxUploadBytes): a non-zero cap rejects an oversized object and
// cleans it up, rather than letting it start the AI pipeline.
func TestConfirmUploadUseCase_ConfirmUpload_RejectsOversizedUploadInDemoMode(t *testing.T) {
	repo := &fakeRepository{meeting: &entity.Meeting{
		ID: "meeting-1", OrgID: "org-1", RecordingObjectKey: "org-1/meeting-1/rec.webm", Status: entity.StatusUploaded,
	}}
	publisher := &fakePublisher{}
	store := &fakeStorage{size: 30 * 1024 * 1024}
	uc := usecase.NewConfirmUploadUseCase(repo, store, publisher, logger.New("test", logger.LevelError), 25*1024*1024)

	if _, err := uc.ConfirmUpload(context.Background(), "org-1", "meeting-1"); err == nil {
		t.Fatal("expected an error for an upload over the demo-mode size cap")
	}
	if len(publisher.uploaded) != 0 {
		t.Fatalf("expected no meeting.uploaded.v1 event for a rejected upload, got %+v", publisher.uploaded)
	}
	if len(store.deleted) != 1 || store.deleted[0] != "org-1/meeting-1/rec.webm" {
		t.Fatalf("expected the oversized object to be deleted, got %+v", store.deleted)
	}
}

// A size at or under the cap is unaffected — this guards against an
// off-by-one that would reject exactly-at-the-limit uploads too.
func TestConfirmUploadUseCase_ConfirmUpload_AllowsUploadAtTheCap(t *testing.T) {
	repo := &fakeRepository{meeting: &entity.Meeting{
		ID: "meeting-1", OrgID: "org-1", RecordingObjectKey: "org-1/meeting-1/rec.webm", Status: entity.StatusUploaded,
	}}
	publisher := &fakePublisher{}
	store := &fakeStorage{size: 25 * 1024 * 1024}
	uc := usecase.NewConfirmUploadUseCase(repo, store, publisher, logger.New("test", logger.LevelError), 25*1024*1024)

	if _, err := uc.ConfirmUpload(context.Background(), "org-1", "meeting-1"); err != nil {
		t.Fatalf("ConfirmUpload: %v", err)
	}
	if len(publisher.uploaded) != 1 {
		t.Fatalf("expected one meeting.uploaded.v1 event, got %d", len(publisher.uploaded))
	}
}
