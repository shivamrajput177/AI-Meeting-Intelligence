package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/storage"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
)

// UpdateStatusUseCase is the "debug endpoint" docs/ROADMAP.md Phase 1
// calls for manually flipping a meeting's status, standing in for the
// Kafka-driven status machine that doesn't exist until Phase 2 — see
// docs/architecture/microservices.md §5.
type UpdateStatusUseCase struct{ repo repository.Repository }

func NewUpdateStatusUseCase(repo repository.Repository) *UpdateStatusUseCase {
	return &UpdateStatusUseCase{repo}
}

func (uc *UpdateStatusUseCase) UpdateStatus(ctx context.Context, orgID, id, status string) (*entity.Meeting, error) {
	if !entity.ValidStatuses[status] {
		return nil, apperr.BadRequest("invalid status")
	}
	if err := uc.repo.UpdateStatus(ctx, orgID, id, status); err != nil {
		return nil, err
	}
	return uc.repo.GetByID(ctx, orgID, id)
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
