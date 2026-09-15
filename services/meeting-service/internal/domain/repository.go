package domain

import "context"

type ListFilter struct {
	Page     int
	PageSize int
}

type Repository interface {
	Create(ctx context.Context, m *Meeting) error
	GetByID(ctx context.Context, orgID, id string) (*Meeting, error)
	List(ctx context.Context, orgID string, filter ListFilter) (items []*Meeting, total int, err error)
	UpdateStatus(ctx context.Context, orgID, id, status string) error
	Touch(ctx context.Context, orgID, id string) error
	Delete(ctx context.Context, orgID, id string) error
}

// ObjectStorage is the port over MinIO — see
// storage/minio for the real implementation.
type ObjectStorage interface {
	PresignedPutURL(ctx context.Context, objectKey string) (url string, err error)
	Stat(ctx context.Context, objectKey string) error
	Delete(ctx context.Context, objectKey string) error
}
