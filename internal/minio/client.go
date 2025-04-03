package minio

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
)

// Client defines the interface for MinIO operations
type Client interface {
	UploadImage(ctx context.Context, reader io.Reader, objectName string, contentType string) error
	GetImage(ctx context.Context, objectName string) (io.ReadCloser, error)
	DeleteImage(ctx context.Context, objectName string) error
	GetImageURL(ctx context.Context, objectName string, expires time.Duration) (string, error)
	GenerateObjectName(id uuid.UUID, fileName string) string

	// Close closes the MinIO client connection
	Close() error
}
