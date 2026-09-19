// Package minio implements storage.ObjectStorage read-only against the
// same MinIO bucket Meeting Service writes uploaded recordings to. Only
// one endpoint here, unlike meeting-service/storage/minio's
// internal/public split: this service never presigns a URL for a
// browser, so there's no "what hostname can a browser resolve" concern —
// every call is server-to-server, always through the internal endpoint.
package minio

import (
	"context"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
	client *minio.Client
	bucket string
}

func New(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*Client, error) {
	c, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, err
	}
	return &Client{client: c, bucket: bucket}, nil
}

func (c *Client) GetObject(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	return c.client.GetObject(ctx, c.bucket, objectKey, minio.GetObjectOptions{})
}
