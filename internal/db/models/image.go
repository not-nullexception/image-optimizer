package models

import (
	"time"

	"github.com/google/uuid"
)

type ProcessingStatus string

const (
	StatusPending    ProcessingStatus = "pending"
	StatusProcessing ProcessingStatus = "processing"
	StatusCompleted  ProcessingStatus = "completed"
	StatusFailed     ProcessingStatus = "failed"
)

// Image represents an image in the system
type Image struct {
	ID              uuid.UUID        `json:"id" db:"id"`
	OriginalName    string           `json:"original_name" db:"original_name"`
	OriginalSize    int64            `json:"original_size" db:"original_size"`
	OriginalWidth   int              `json:"original_width" db:"original_width"`
	OriginalHeight  int              `json:"original_height" db:"original_height"`
	OriginalFormat  string           `json:"original_format" db:"original_format"`
	OriginalPath    string           `json:"original_path" db:"original_path"`
	OptimizedPath   string           `json:"optimized_path,omitempty" db:"optimized_path"`
	OptimizedSize   int64            `json:"optimized_size,omitempty" db:"optimized_size"`
	OptimizedWidth  int              `json:"optimized_width,omitempty" db:"optimized_width"`
	OptimizedHeight int              `json:"optimized_height,omitempty" db:"optimized_height"`
	Status          ProcessingStatus `json:"status" db:"status"`
	Error           string           `json:"error,omitempty" db:"error"`
	CreatedAt       time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at" db:"updated_at"`
}

// NewImage creates a new Image with default values
func NewImage(originalName string, originalSize int64, originalWidth, originalHeight int, originalFormat, originalPath string) *Image {
	now := time.Now()
	return &Image{
		ID:             uuid.New(),
		OriginalName:   originalName,
		OriginalSize:   originalSize,
		OriginalWidth:  originalWidth,
		OriginalHeight: originalHeight,
		OriginalFormat: originalFormat,
		OriginalPath:   originalPath,
		Status:         StatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// ImageListResponse represents the response for image listing
type ImageListResponse struct {
	Images []*Image `json:"images"`
	Total  int      `json:"total"`
}

// ImageResponse represents the response for a single image
type ImageResponse struct {
	ID            uuid.UUID        `json:"id"`
	OriginalName  string           `json:"original_name"`
	Status        ProcessingStatus `json:"status"`
	OriginalURL   string           `json:"original_url,omitempty"`
	OptimizedURL  string           `json:"optimized_url,omitempty"`
	OriginalSize  int64            `json:"original_size"`
	OptimizedSize int64            `json:"optimized_size,omitempty"`
	Reduction     float64          `json:"reduction,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
	Error         string           `json:"error,omitempty"`
}

// ImageUploadResponse represents the response for image upload
type ImageUploadResponse struct {
	ID     uuid.UUID `json:"id"`
	Status string    `json:"status"`
}
