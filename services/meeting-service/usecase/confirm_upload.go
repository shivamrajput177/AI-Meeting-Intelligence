package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/storage"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
)

type ConfirmUploadUseCase struct {
	repo    repository.Repository
	storage storage.ObjectStorage
}

func NewConfirmUploadUseCase(repo repository.Repository, storage storage.ObjectStorage) *ConfirmUploadUseCase {
	return &ConfirmUploadUseCase{repo: repo, storage: storage}
}

// ConfirmUpload verifies the object actually landed in MinIO before treating
// the upload as real — a client that calls this without ever PUTting the
// file would otherwise leave a meeting row pointing at nothing.
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
	return uc.repo.GetByID(ctx, orgID, meetingID)
}
