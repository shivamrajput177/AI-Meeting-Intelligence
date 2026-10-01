package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/events"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/storage"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/metrics"
)

// UpdateStatusUseCase is shared by two callers with very different
// trust models: PATCH /meetings/{id}/status, the "debug endpoint"
// docs/ROADMAP.md Phase 1 describes for manually flipping a meeting's
// status (still owner-only, see handler.UpdateStatus), and, since
// Phase 2.6, consumer.go's Kafka-driven status machine advancing a
// meeting's status off transcription/summary/action-item
// completion/failure events — see docs/architecture/microservices.md §5.
type UpdateStatusUseCase struct {
	repo      repository.Repository
	publisher events.Publisher
	log       *logger.Logger
}

func NewUpdateStatusUseCase(repo repository.Repository, publisher events.Publisher, log *logger.Logger) *UpdateStatusUseCase {
	return &UpdateStatusUseCase{repo: repo, publisher: publisher, log: log}
}

// UpdateStatus persists the new status, then best-effort publishes
// meeting.status-changed.v1 — same trade-off as ConfirmUpload's own
// publish (no transactional outbox yet, so a dropped event here is a
// real, logged gap rather than a silently swallowed one, and the durable
// Postgres write already succeeded regardless).
func (uc *UpdateStatusUseCase) UpdateStatus(ctx context.Context, orgID, id, status string) (*entity.Meeting, error) {
	if !entity.ValidStatuses[status] {
		return nil, apperr.BadRequest("invalid status")
	}
	if err := uc.repo.UpdateStatus(ctx, orgID, id, status); err != nil {
		return nil, err
	}
	meeting, err := uc.repo.GetByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}

	if status == entity.StatusCompleted {
		metrics.MeetingsProcessedTotal.Inc()
	}

	if err := uc.publisher.PublishMeetingStatusChanged(ctx, entity.MeetingStatusChangedEvent{
		MeetingID: meeting.ID, OrgID: meeting.OrgID, Status: meeting.Status,
	}); err != nil {
		uc.log.Error("publish meeting.status-changed.v1 failed", "meeting_id", meeting.ID, "err", err)
	}
	return meeting, nil
}

type DeleteMeetingUseCase struct {
	repo    repository.Repository
	storage storage.ObjectStorage
}

func NewDeleteMeetingUseCase(repo repository.Repository, storage storage.ObjectStorage) *DeleteMeetingUseCase {
	return &DeleteMeetingUseCase{repo: repo, storage: storage}
}

func (uc *DeleteMeetingUseCase) DeleteMeeting(ctx context.Context, orgID, id string) error {
	meeting, err := uc.repo.GetByID(ctx, orgID, id)
	if err != nil {
		return err
	}
	// Best-effort object deletion — a MinIO hiccup shouldn't block the
	// user from deleting the meeting row; an orphaned object is a cheap,
	// recoverable cost (a lifecycle-policy sweep in a real deployment),
	// unlike a meeting the user can't delete at all.
	_ = uc.storage.Delete(ctx, meeting.RecordingObjectKey)
	return uc.repo.Delete(ctx, orgID, id)
}
