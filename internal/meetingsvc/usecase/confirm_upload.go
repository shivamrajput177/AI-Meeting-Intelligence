package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/meetingsvc/domain"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/apperr"
)

type ConfirmUploadUseCase struct {
	repo    domain.Repository
	storage domain.ObjectStorage
}

func NewConfirmUploadUseCase(repo domain.Repository, storage domain.ObjectStorage) *ConfirmUploadUseCase {
	return &ConfirmUploadUseCase{repo: repo, storage: storage}
}

// Execute verifies the object actually landed in MinIO before treating
// the upload as real — a client that calls this without ever PUTting the
// file would otherwise leave a meeting row pointing at nothing.
func (uc *ConfirmUploadUseCase) Execute(ctx context.Context, orgID, meetingID string) (*domain.Meeting, error) {
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
	return uc.repo.GetByID(ctx, orgID, meetingID)
}
