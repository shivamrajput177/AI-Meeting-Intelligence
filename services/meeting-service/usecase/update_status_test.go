package usecase_test

import (
	"context"
	"sync"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// fakeRepository is an in-memory stand-in for the real Postgres
// repository — this is the whole point of defining repository.Repository
// as an interface usecase depends on (see
// docs/architecture/folder-structure.md's Clean Architecture layering):
// UpdateStatusUseCase is fully testable with zero Postgres involved. Only
// the methods UpdateStatus actually calls (UpdateStatus, GetByID) do
// anything; the rest exist only to satisfy repository.Repository.
type fakeRepository struct {
	mu      sync.Mutex
	meeting *entity.Meeting
}

func (f *fakeRepository) Create(context.Context, *entity.Meeting) error { return nil }
func (f *fakeRepository) GetByID(_ context.Context, _, _ string) (*entity.Meeting, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.meeting, nil
}
func (f *fakeRepository) List(context.Context, string, entity.ListFilter) ([]*entity.Meeting, int, error) {
	return nil, 0, nil
}
func (f *fakeRepository) UpdateStatus(_ context.Context, _, _, status string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.meeting.Status = status
	return nil
}
func (f *fakeRepository) Touch(context.Context, string, string) error  { return nil }
func (f *fakeRepository) Delete(context.Context, string, string) error { return nil }
func (f *fakeRepository) ListParticipants(context.Context, string, string) ([]*entity.Participant, error) {
	return nil, nil
}

// fakePublisher is an in-memory stand-in for the real Kafka producer.
type fakePublisher struct {
	mu            sync.Mutex
	statusChanged []entity.MeetingStatusChangedEvent
}

func (f *fakePublisher) PublishMeetingUploaded(context.Context, entity.MeetingUploadedEvent) error {
	return nil
}
func (f *fakePublisher) PublishMeetingStatusChanged(_ context.Context, event entity.MeetingStatusChangedEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusChanged = append(f.statusChanged, event)
	return nil
}

func TestUpdateStatusUseCase_UpdateStatus(t *testing.T) {
	repo := &fakeRepository{meeting: &entity.Meeting{ID: "meeting-1", OrgID: "org-1", Status: entity.StatusUploaded}}
	publisher := &fakePublisher{}
	uc := usecase.NewUpdateStatusUseCase(repo, publisher, logger.New("test", logger.LevelError))

	meeting, err := uc.UpdateStatus(context.Background(), "org-1", "meeting-1", entity.StatusTranscribed)
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if meeting.Status != entity.StatusTranscribed {
		t.Fatalf("Status = %q, want %q", meeting.Status, entity.StatusTranscribed)
	}
	if len(publisher.statusChanged) != 1 || publisher.statusChanged[0].Status != entity.StatusTranscribed {
		t.Fatalf("expected one meeting.status-changed.v1 event with status %q, got %+v", entity.StatusTranscribed, publisher.statusChanged)
	}
	if publisher.statusChanged[0].MeetingID != "meeting-1" || publisher.statusChanged[0].OrgID != "org-1" {
		t.Fatalf("unexpected event meeting/org id: %+v", publisher.statusChanged[0])
	}
}

func TestUpdateStatusUseCase_UpdateStatus_InvalidStatus(t *testing.T) {
	repo := &fakeRepository{meeting: &entity.Meeting{ID: "meeting-1", OrgID: "org-1", Status: entity.StatusUploaded}}
	publisher := &fakePublisher{}
	uc := usecase.NewUpdateStatusUseCase(repo, publisher, logger.New("test", logger.LevelError))

	if _, err := uc.UpdateStatus(context.Background(), "org-1", "meeting-1", "not-a-real-status"); err == nil {
		t.Fatal("expected an error for an invalid status")
	}
	if len(publisher.statusChanged) != 0 {
		t.Fatalf("expected no event published for a rejected status update, got %+v", publisher.statusChanged)
	}
}
