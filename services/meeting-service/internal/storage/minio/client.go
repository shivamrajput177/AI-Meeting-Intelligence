// Package minio implements meetingsvc/domain.ObjectStorage against MinIO
// — see docs/architecture/microservices.md §5 and the "What is MinIO"
// explanation in the project's design docs: an S3-compatible, self-hosted
// object store for the actual recording bytes, so Postgres only ever
// holds metadata.
package minio

import (
	"context"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Client wraps two minio.Client instances pointed at the *same* MinIO
// server under two different hostnames — a well-known Docker networking
// gotcha this deliberately works around:
//
//   - internal, reached at the Docker Compose service name (e.g.
//     "minio:9000"), used for server-side operations this service performs
//     itself (bucket setup, stat, delete).
//   - public, reached at whatever hostname/port a *browser* on the host
//     machine can actually resolve (e.g. "localhost:9000", from the
//     published port in docker-compose.yaml), used only to presign PUT
//     URLs. Presigning is a local, offline crypto operation (the signature
//     covers the configured host, not a live round trip), so a client
//     configured with a host it never actually connects to still produces
//     a correct, usable URL for whoever the URL is handed to.
//
// Skipping this split is the single most common "why does my presigned
// upload URL 404/connection-refuse in the browser" mistake when MinIO
// runs in Docker — worth documenting explicitly rather than discovering
// it by trial and error.
type Client struct {
	internal *minio.Client
	public   *minio.Client
	bucket   string
}

func New(internalEndpoint, publicEndpoint, accessKey, secretKey, bucket string, useSSL bool) (*Client, error) {
	creds := credentials.NewStaticV4(accessKey, secretKey, "")

	internal, err := minio.New(internalEndpoint, &minio.Options{Creds: creds, Secure: useSSL})
	if err != nil {
		return nil, err
	}
	public, err := minio.New(publicEndpoint, &minio.Options{Creds: creds, Secure: useSSL})
	if err != nil {
		return nil, err
	}
	return &Client{internal: internal, public: public, bucket: bucket}, nil
}

// EnsureBucket creates the recordings bucket if it doesn't exist yet —
// called once at service startup so `docker compose up` alone is enough,
// with no separate `mc mb` init step for a developer to remember.
func (c *Client) EnsureBucket(ctx context.Context) error {
	exists, err := c.internal.BucketExists(ctx, c.bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return c.internal.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{})
}

// PresignedPutURL returns a short-lived URL the client uploads the
// recording bytes to directly — Meeting Service never proxies the file
// itself, per docs/architecture/microservices.md §5. Built via the
// public-facing client (see Client's doc comment).
func (c *Client) PresignedPutURL(ctx context.Context, objectKey string) (string, error) {
	u, err := c.public.PresignedPutObject(ctx, c.bucket, objectKey, 15*time.Minute)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (c *Client) Stat(ctx context.Context, objectKey string) error {
	_, err := c.internal.StatObject(ctx, c.bucket, objectKey, minio.StatObjectOptions{})
	return err
}

func (c *Client) Delete(ctx context.Context, objectKey string) error {
	return c.internal.RemoveObject(ctx, c.bucket, objectKey, minio.RemoveObjectOptions{})
}
