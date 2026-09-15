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
	Stat(ctx context.Context, objectKey string) error
	Delete(ctx context.Context, objectKey string) error
}
