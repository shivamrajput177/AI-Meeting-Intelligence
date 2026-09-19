// Package storage defines the ObjectStorage interface usecase depends
// on; storage/minio, right below this package in the same tree,
// implements it against MinIO — see that package's doc comment. Keeping
// the interface here instead of off in some unrelated package is just
// where it belongs — its one real implementation lives one directory
// down.
package storage

import (
	"context"
	"io"
)

// ObjectStorage is read-only here, unlike meeting-service's own storage
// interface: this service only ever fetches a recording another service
// already wrote, never presigns an upload or deletes anything.
type ObjectStorage interface {
	GetObject(ctx context.Context, objectKey string) (io.ReadCloser, error)
}
