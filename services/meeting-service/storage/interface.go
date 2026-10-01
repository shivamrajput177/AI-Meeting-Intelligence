// Package storage defines the ObjectStorage interface usecase depends
// on; storage/minio, right below this package in the same tree,
// implements it against MinIO — see that package's doc comment. Keeping
// the interface here instead of off in some unrelated package is just
// where it belongs — its one real implementation lives one directory
// down.
package storage

import "context"

type ObjectStorage interface {
	PresignedPutURL(ctx context.Context, objectKey string) (url string, err error)
	// Stat returns the object's size in bytes — ConfirmUploadUseCase uses
	// it both to confirm the upload landed at all and, in the public demo
	// deployment (Phase 7), to enforce a size cap as a practical proxy for
	// a clip-length limit. See that usecase's doc comment for why.
	Stat(ctx context.Context, objectKey string) (sizeBytes int64, err error)
	Delete(ctx context.Context, objectKey string) error
}
