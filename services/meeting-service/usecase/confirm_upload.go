package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/events"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/storage"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

type ConfirmUploadUseCase struct {
	repo      repository.Repository
	storage   storage.ObjectStorage
	publisher events.Publisher
	log       *logger.Logger
}

func NewConfirmUploadUseCase(repo repository.Repository, storage storage.ObjectStorage, publisher events.Publisher, log *logger.Logger) *ConfirmUploadUseCase {
	return &ConfirmUploadUseCase{repo: repo, storage: storage, publisher: publisher, log: log}
}

// ConfirmUpload verifies the object actually landed in MinIO before treating
// the upload as real — a client that calls this without ever PUTting the
// file would otherwise leave a meeting row pointing at nothing. Once
// confirmed, it publishes meeting.uploaded.v1 to kick off the Phase 2
// pipeline (see docs/architecture/kafka-topics.md's event-flow diagram).
func (uc *ConfirmUploadUseCase) ConfirmUpload(ctx context.Context, orgID, meetingID string) (*entity.Meeting, error) {
	meeting, err := uc.repo.GetByID(ctx, orgID, meetingID)
	if err != nil {
		return nil, err
	}
	if err := uc.storage.Stat(ctx, meeting.RecordingObjectKey); err != nil {
		return nil, apperr.BadRequest("recording not found — did the upload complete?")
	}
	if err := uc.repo.Touch(ctx, orgID, meetingID); err != nil {
		return nil, apperr.Internal("update meeting").Wrap(err)
	}
	updated, err := uc.repo.GetByID(ctx, orgID, meetingID)
	if err != nil {
		return nil, err
	}

	// Best-effort publish: Kafka being down shouldn't fail a request that
	// already durably confirmed the upload in Postgres+MinIO. There's no
	// transactional outbox yet (Phase 4) to make this delivery guaranteed
	// — the same kind of known gap as Signup's cross-service
	// orchestration in auth-service's SignupUseCase — so a dropped event
	// here means the meeting sits in "uploaded" until a future
	// retry/reconciliation mechanism exists. Logged loudly rather than
	// silently swallowed, since it's a real gap, not a non-issue.
	if err := uc.publisher.PublishMeetingUploaded(ctx, entity.MeetingUploadedEvent{
		MeetingID: updated.ID, OrgID: updated.OrgID, Title: updated.Title,
		SourceType: updated.SourceType, RecordingObjectKey: updated.RecordingObjectKey,
	}); err != nil {
		uc.log.Error("publish meeting.uploaded.v1 failed", "meeting_id", updated.ID, "err", err)
	}
	return updated, nil
}
