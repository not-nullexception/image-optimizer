package db

import (
	"context"

	"github.com/google/uuid"
	"github.com/not-nullexception/image-optimizer/internal/db/models"
)

// Repository defines the interface for database operations
type Repository interface {
	GetImageByID(ctx context.Context, id uuid.UUID) (*models.Image, error)
	ListImages(ctx context.Context, limit, offset int) ([]*models.Image, int, error)
	CreateImage(ctx context.Context, image *models.Image) error
	UpdateImage(ctx context.Context, image *models.Image) error
	DeleteImage(ctx context.Context, id uuid.UUID) error
	UpdateImageStatus(ctx context.Context, id uuid.UUID, status models.ProcessingStatus, errorMsg string) error
	UpdateImageOptimized(ctx context.Context, id uuid.UUID, path string, size int64, width, height int) error

	// Health check
	Ping(ctx context.Context) error

	// Close the repository
	Close() error
}
