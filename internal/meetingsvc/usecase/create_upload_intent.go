package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/meetingsvc/domain"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/apperr"
)

type CreateUploadIntentInput struct {
	OrgID     string
	CreatedBy string
	Title     string
}

type CreateUploadIntentOutput struct {
	MeetingID string
	UploadURL string
}

type CreateUploadIntentUseCase struct {
	repo    domain.Repository
	storage domain.ObjectStorage
}

func NewCreateUploadIntentUseCase(repo domain.Repository, storage domain.ObjectStorage) *CreateUploadIntentUseCase {
	return &CreateUploadIntentUseCase{repo: repo, storage: storage}
}

// Execute creates the meeting row and hands back a presigned PUT URL —
// the recording's bytes go straight from the browser to MinIO, never
// through this service, per docs/architecture/microservices.md §5.
func (uc *CreateUploadIntentUseCase) Execute(ctx context.Context, in CreateUploadIntentInput) (*CreateUploadIntentOutput, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, apperr.BadRequest("title is required")
	}

	meetingID := uuid.NewString()
	// Object key namespaced org_id/meeting_id — see
	// docs/architecture/observability-security.md §3's multi-tenancy
	// table: MinIO isolation is by key prefix, not by bucket-per-tenant.
	objectKey := fmt.Sprintf("%s/%s/recording", in.OrgID, meetingID)

	uploadURL, err := uc.storage.PresignedPutURL(ctx, objectKey)
	if err != nil {
		return nil, apperr.Internal("presign upload url").Wrap(err)
	}

	now := time.Now()
	meeting := &domain.Meeting{
		ID: meetingID, OrgID: in.OrgID, Title: title, CreatedBy: in.CreatedBy,
		Status: domain.StatusUploaded, SourceType: "upload", RecordingObjectKey: objectKey,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := uc.repo.Create(ctx, meeting); err != nil {
		return nil, apperr.Internal("create meeting").Wrap(err)
	}

	return &CreateUploadIntentOutput{MeetingID: meetingID, UploadURL: uploadURL}, nil
}
