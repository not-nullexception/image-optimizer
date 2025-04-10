package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/not-nullexception/image-optimizer/config"
	"github.com/not-nullexception/image-optimizer/internal/db"
	"github.com/not-nullexception/image-optimizer/internal/db/models"
	"github.com/not-nullexception/image-optimizer/internal/logger"
	"github.com/rs/zerolog"
)

type Repository struct {
	pool   *pgxpool.Pool
	logger zerolog.Logger
}

func NewRepository(ctx context.Context, cfg *config.DatabaseConfig) (db.Repository, error) {
	log := logger.GetLogger("postgres-repository")

	// Create a connection pool configuration
	poolConfig, err := pgxpool.ParseConfig(cfg.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("unable to parse connection string: %w", err)
	}

	// Set pool configuration
	poolConfig.MaxConns = int32(cfg.MaxConnections)
	poolConfig.MinConns = int32(cfg.MinConnections)

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}

	log.Info().Msg("Connected to Postgres database")
	return &Repository{pool: pool, logger: log}, nil
}

// GetImageByID retrieves an image by its ID
func (r *Repository) GetImageByID(ctx context.Context, id uuid.UUID) (*models.Image, error) {
	query := `
		SELECT id, original_name, original_size, original_width, original_height,
			original_format, original_path, optimized_path, optimized_size,
			optimized_width, optimized_height, status, error, created_at, updated_at
		FROM images
		WHERE id = $1
	`
	var img models.Image
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&img.ID, &img.OriginalName, &img.OriginalSize, &img.OriginalWidth, &img.OriginalHeight,
		&img.OriginalFormat, &img.OriginalPath, &img.OptimizedPath, &img.OptimizedSize,
		&img.OptimizedWidth, &img.OptimizedHeight, &img.Status, &img.Error, &img.CreatedAt, &img.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("image not found: %w", err)
		}
		return nil, fmt.Errorf("error querying image: %w", err)
	}

	return &img, nil
}

// ListImages retrieves a list of images with pagination
func (r *Repository) ListImages(ctx context.Context, limit, offset int) ([]*models.Image, int, error) {
	query := `
		SELECT id, original_name, original_size, original_width, original_height, 
			original_format, original_path, optimized_path, optimized_size, 
			optimized_width, optimized_height, status, error, created_at, updated_at
		FROM images
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	countQuery := `SELECT COUNT(*) FROM images`

	// Get the total count
	var total int
	err := r.pool.QueryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("error counting images: %w", err)
	}

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("error querying images: %w", err)
	}
	defer rows.Close()

	images := make([]*models.Image, 0)
	for rows.Next() {
		var img models.Image
		err := rows.Scan(
			&img.ID, &img.OriginalName, &img.OriginalSize, &img.OriginalWidth, &img.OriginalHeight,
			&img.OriginalFormat, &img.OriginalPath, &img.OptimizedPath, &img.OptimizedSize,
			&img.OptimizedWidth, &img.OptimizedHeight, &img.Status, &img.Error, &img.CreatedAt, &img.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("error scanning image row: %w", err)
		}
		images = append(images, &img)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating over rows: %w", err)
	}

	return images, total, nil
}

// CreateImage creates a new image record
func (r *Repository) CreateImage(ctx context.Context, image *models.Image) error {
	query := `
		INSERT INTO images (
			id, original_name, original_size, original_width, original_height,
			original_format, original_path, status, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
	`

	_, err := r.pool.Exec(ctx, query,
		image.ID, image.OriginalName, image.OriginalSize, image.OriginalWidth, image.OriginalHeight,
		image.OriginalFormat, image.OriginalPath, image.Status, image.CreatedAt, image.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("error creating image: %w", err)
	}

	return nil
}

// UpdateImage updates an existing image record
func (r *Repository) UpdateImage(ctx context.Context, image *models.Image) error {
	query := `
		UPDATE images
		SET original_name = $2, original_size = $3, original_width = $4, original_height = $5,
			original_format = $6, original_path = $7, optimized_path = $8, optimized_size = $9,
			optimized_width = $10, optimized_height = $11, status = $12, error = $13, updated_at = $14
		WHERE id = $1
	`

	image.UpdatedAt = time.Now()

	_, err := r.pool.Exec(ctx, query,
		image.ID, image.OriginalName, image.OriginalSize, image.OriginalWidth, image.OriginalHeight,
		image.OriginalFormat, image.OriginalPath, image.OptimizedPath, image.OptimizedSize,
		image.OptimizedWidth, image.OptimizedHeight, image.Status, image.Error, image.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("error updating image: %w", err)
	}

	return nil
}

// DeleteImage deletes an image record
func (r *Repository) DeleteImage(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM images WHERE id = $1`

	commandTag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("error deleting image: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return fmt.Errorf("image not found")
	}

	return nil
}

// UpdateImageStatus updates the status of an image
func (r *Repository) UpdateImageStatus(ctx context.Context, id uuid.UUID, status models.ProcessingStatus, errorMsg string) error {
	query := `
		UPDATE images
		SET status = $2, error = $3, updated_at = $4
		WHERE id = $1
	`

	updatedAt := time.Now()

	_, err := r.pool.Exec(ctx, query, id, status, errorMsg, updatedAt)
	if err != nil {
		return fmt.Errorf("error updating image status: %w", err)
	}

	return nil
}

// UpdateImageOptimized updates the optimized image information
func (r *Repository) UpdateImageOptimized(ctx context.Context, id uuid.UUID, path string, size int64, width, height int) error {
	query := `
		UPDATE images
		SET optimized_path = $2, optimized_size = $3, optimized_width = $4, optimized_height = $5,
			status = $6, updated_at = $7
		WHERE id = $1
	`

	updatedAt := time.Now()

	_, err := r.pool.Exec(ctx, query,
		id, path, size, width, height,
		models.StatusCompleted, updatedAt,
	)
	if err != nil {
		return fmt.Errorf("error updating optimized image: %w", err)
	}

	return nil
}

func (r *Repository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

func (r *Repository) Close() error {
	r.pool.Close()
	return nil
}
