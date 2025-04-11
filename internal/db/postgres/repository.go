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
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(ctx context.Context, cfg *config.DatabaseConfig) (db.Repository, error) {
	initLogger := logger.GetLogger("postgres-repository")

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

	initLogger.Info().Msg("Connected to Postgres database")
	return &Repository{pool: pool}, nil
}

// GetImageByID retrieves an image by its ID
func (r *Repository) GetImageByID(ctx context.Context, id uuid.UUID) (*models.Image, error) {
	reqLogger := logger.FromContext(ctx)

	query := `
		SELECT id, original_name, original_size, original_width, original_height,
			original_format, original_path, optimized_path, optimized_size,
			optimized_width, optimized_height, status, error, created_at, updated_at
		FROM images
		WHERE id = $1
	`

	reqLogger.Debug().Str("image_id", id.String()).Msg("Executing GetImageByID query")

	var img models.Image
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&img.ID, &img.OriginalName, &img.OriginalSize, &img.OriginalWidth, &img.OriginalHeight,
		&img.OriginalFormat, &img.OriginalPath, &img.OptimizedPath, &img.OptimizedSize,
		&img.OptimizedWidth, &img.OptimizedHeight, &img.Status, &img.Error, &img.CreatedAt, &img.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			reqLogger.Warn().Err(err).Str("image_id", id.String()).Msg("Image not found")
			return nil, fmt.Errorf("image not found: %w", err)
		}

		reqLogger.Error().Err(err).Str("image_id", id.String()).Msg("Error querying image")
		return nil, fmt.Errorf("error querying image: %w", err)
	}

	reqLogger.Debug().Str("image_id", id.String()).Msg("Image retrieved successfully")
	return &img, nil
}

// ListImages retrieves a list of images with pagination
func (r *Repository) ListImages(ctx context.Context, limit, offset int) ([]*models.Image, int, error) {
	reqLogger := logger.FromContext(ctx)

	query := `
		SELECT id, original_name, original_size, original_width, original_height, 
			original_format, original_path, optimized_path, optimized_size, 
			optimized_width, optimized_height, status, error, created_at, updated_at
		FROM images
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	countQuery := `SELECT COUNT(*) FROM images`

	reqLogger.Debug().Int("limit", limit).Int("offset", offset).Msg("Executing ListImages query")

	var total int
	err := r.pool.QueryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		reqLogger.Error().Err(err).Msg("Error counting images")
		return nil, 0, fmt.Errorf("error counting images: %w", err)
	}

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		reqLogger.Error().Err(err).Msg("Error querying images")
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
			reqLogger.Error().Err(err).Msg("Error scanning image row")
			return nil, 0, fmt.Errorf("error scanning image row: %w", err)
		}
		images = append(images, &img)
	}

	if err := rows.Err(); err != nil {
		reqLogger.Error().Err(err).Msg("Error iterating over image rows")
		return nil, 0, fmt.Errorf("error iterating over rows: %w", err)
	}

	reqLogger.Debug().Int("total_images", total).Msg("Total images retrieved")
	return images, total, nil
}

// CreateImage creates a new image record
func (r *Repository) CreateImage(ctx context.Context, image *models.Image) error {
	reqLogger := logger.FromContext(ctx)

	query := `
		INSERT INTO images (
			id, original_name, original_size, original_width, original_height,
			original_format, original_path, status, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
	`

	reqLogger.Debug().Str("image_id", image.ID.String()).Msg("Executing CreateImage query")

	_, err := r.pool.Exec(ctx, query,
		image.ID, image.OriginalName, image.OriginalSize, image.OriginalWidth, image.OriginalHeight,
		image.OriginalFormat, image.OriginalPath, image.Status, image.CreatedAt, image.UpdatedAt,
	)

	if err != nil {
		reqLogger.Error().Err(err).Msg("Error creating image")
		return fmt.Errorf("error creating image: %w", err)
	}

	reqLogger.Debug().Str("image_id", image.ID.String()).Msg("Image created successfully")
	return nil
}

// UpdateImage updates an existing image record
func (r *Repository) UpdateImage(ctx context.Context, image *models.Image) error {
	reqLogger := logger.FromContext(ctx)

	query := `
		UPDATE images
		SET original_name = $2, original_size = $3, original_width = $4, original_height = $5,
			original_format = $6, original_path = $7, optimized_path = $8, optimized_size = $9,
			optimized_width = $10, optimized_height = $11, status = $12, error = $13, updated_at = $14
		WHERE id = $1
	`

	reqLogger.Debug().Str("image_id", image.ID.String()).Msg("Executing UpdateImage query")

	image.UpdatedAt = time.Now()

	_, err := r.pool.Exec(ctx, query,
		image.ID, image.OriginalName, image.OriginalSize, image.OriginalWidth, image.OriginalHeight,
		image.OriginalFormat, image.OriginalPath, image.OptimizedPath, image.OptimizedSize,
		image.OptimizedWidth, image.OptimizedHeight, image.Status, image.Error, image.UpdatedAt,
	)

	if err != nil {
		reqLogger.Error().Err(err).Msg("Error updating image")
		return fmt.Errorf("error updating image: %w", err)
	}

	reqLogger.Debug().Str("image_id", image.ID.String()).Msg("Image updated successfully")
	return nil
}

// DeleteImage deletes an image record
func (r *Repository) DeleteImage(ctx context.Context, id uuid.UUID) error {
	reqLogger := logger.FromContext(ctx)

	query := `DELETE FROM images WHERE id = $1`

	reqLogger.Debug().Str("image_id", id.String()).Msg("Executing DeleteImage query")

	commandTag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		reqLogger.Error().Err(err).Msg("Error deleting image")
		return fmt.Errorf("error deleting image: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		reqLogger.Warn().Str("image_id", id.String()).Msg("Image not found for deletion")
		return fmt.Errorf("image not found")
	}

	reqLogger.Debug().Str("image_id", id.String()).Msg("Image deleted successfully")
	return nil
}

// UpdateImageStatus updates the status of an image
func (r *Repository) UpdateImageStatus(ctx context.Context, id uuid.UUID, status models.ProcessingStatus, errorMsg string) error {
	reqLogger := logger.FromContext(ctx)

	query := `
		UPDATE images
		SET status = $2, error = $3, updated_at = $4
		WHERE id = $1
	`

	reqLogger.Debug().Str("image_id", id.String()).Msg("Executing UpdateImageStatus query")

	updatedAt := time.Now()

	_, err := r.pool.Exec(ctx, query, id, status, errorMsg, updatedAt)
	if err != nil {
		reqLogger.Error().Err(err).Msg("Error updating image status")
		return fmt.Errorf("error updating image status: %w", err)
	}

	reqLogger.Debug().Str("image_id", id.String()).Msg("Image status updated successfully")
	return nil
}

// UpdateImageOptimized updates the optimized image information
func (r *Repository) UpdateImageOptimized(ctx context.Context, id uuid.UUID, path string, size int64, width, height int) error {
	reqLogger := logger.FromContext(ctx)

	query := `
		UPDATE images
		SET optimized_path = $2, optimized_size = $3, optimized_width = $4, optimized_height = $5,
			status = $6, updated_at = $7
		WHERE id = $1
	`

	reqLogger.Debug().Str("image_id", id.String()).Msg("Executing UpdateImageOptimized query")

	updatedAt := time.Now()

	_, err := r.pool.Exec(ctx, query,
		id, path, size, width, height,
		models.StatusCompleted, updatedAt,
	)
	if err != nil {
		reqLogger.Error().Err(err).Msg("Error updating optimized image")
		return fmt.Errorf("error updating optimized image: %w", err)
	}

	reqLogger.Debug().Str("image_id", id.String()).Msg("Optimized image updated successfully")
	return nil
}

func (r *Repository) Ping(ctx context.Context) error {
	reqLogger := logger.FromContext(ctx)
	reqLogger.Debug().Msg("Pinging database")

	err := r.pool.Ping(ctx)
	if err != nil {
		reqLogger.Error().Err(err).Msg("Error pinging database")
		return fmt.Errorf("error pinging database: %w", err)
	}

	return nil
}

func (r *Repository) Close() error {
	r.pool.Close()
	return nil
}
