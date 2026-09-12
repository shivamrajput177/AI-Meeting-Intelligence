package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/meetingsvc/domain"
)

// UpdateStatusUseCase is the "debug endpoint" docs/ROADMAP.md Phase 1
// calls for manually flipping a meeting's status, standing in for the
// Kafka-driven status machine that doesn't exist until Phase 2 — see
// docs/architecture/microservices.md §5.
type UpdateStatusUseCase struct{ repo domain.Repository }

func NewUpdateStatusUseCase(repo domain.Repository) *UpdateStatusUseCase {
	return &UpdateStatusUseCase{repo}
}

func (uc *UpdateStatusUseCase) Execute(ctx context.Context, orgID, id, status string) (*domain.Meeting, error) {
	if !domain.ValidStatuses[status] {
		return nil, domain.ErrInvalidStatus
	}
	if err := uc.repo.UpdateStatus(ctx, orgID, id, status); err != nil {
		return nil, err
	}
	return uc.repo.GetByID(ctx, orgID, id)
}

type DeleteMeetingUseCase struct {
	repo    domain.Repository
	storage domain.ObjectStorage
}

func NewDeleteMeetingUseCase(repo domain.Repository, storage domain.ObjectStorage) *DeleteMeetingUseCase {
	return &DeleteMeetingUseCase{repo: repo, storage: storage}
}

func (uc *DeleteMeetingUseCase) Execute(ctx context.Context, orgID, id string) error {
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
