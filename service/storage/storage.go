package storage

import (
	"context"
	"io"
	"time"
)

type VideoStorage interface {
	Upload(ctx context.Context, objectKey string, r io.Reader, size int64, contentType string) error
	Delete(ctx context.Context, objectKey string) error
	GetPresignedURL(objectKey string, expiry time.Duration) (string, error)
}
