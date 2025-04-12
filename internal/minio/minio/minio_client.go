package minio

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	minioLib "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/not-nullexception/image-optimizer/config"
	"github.com/not-nullexception/image-optimizer/internal/logger"
	"github.com/not-nullexception/image-optimizer/internal/minio"
)

type MinioClient struct {
	client     *minioLib.Client
	bucketName string
	config     *config.MinIOConfig
}

func NewClient(cfg *config.MinIOConfig) (minio.Client, error) {
	reqLogger := logger.GetLogger("minio-client")

	// Initialize MinIO client
	client, err := minioLib.New(cfg.Endpoint, &minioLib.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.SSL,
	})
	if err != nil {
		return nil, fmt.Errorf("error initializing MinIO client: %w", err)
	}

	mc := &MinioClient{
		client:     client,
		bucketName: cfg.Bucket,
		config:     cfg,
	}

	exists, err := client.BucketExists(context.Background(), cfg.Bucket)
	if err != nil {
		reqLogger.Error().Err(err).Msg("Error checking if bucket exists")
		return nil, fmt.Errorf("error checking if bucket exists: %w", err)
	}

	if !exists {
		err = client.MakeBucket(context.Background(), cfg.Bucket, minioLib.MakeBucketOptions{Region: cfg.Location})
		if err != nil {
			reqLogger.Error().Err(err).Str("bucket", cfg.Bucket).Msg("Error creating bucket")
			return nil, fmt.Errorf("error creating bucket: %w", err)
		}
		reqLogger.Info().Str("bucket", cfg.Bucket).Msg("Bucket created")
	} else {
		reqLogger.Info().Str("bucket", cfg.Bucket).Msg("Bucket already exists")
	}

	return mc, nil
}

// TODO - Check if we need retry logic with backoff
// UploadImage uploads an image to MinIO
func (m *MinioClient) UploadImage(ctx context.Context, reader io.Reader, objectName string, contentType string) error {
	reqLogger := logger.FromContext(ctx).With().Str("component", "minio-client").Logger()

	reqLogger.Debug().Str("object", objectName).Str("content_type", contentType).Msg("Starting image upload")

	_, err := m.client.PutObject(ctx, m.bucketName, objectName, reader, -1,
		minioLib.PutObjectOptions{ContentType: contentType})
	if err != nil {
		reqLogger.Error().Err(err).Str("object", objectName).Msg("Error uploading image")
		return fmt.Errorf("error uploading image: %w", err)
	}

	reqLogger.Debug().Str("object", objectName).Str("content_type", contentType).Msg("Image uploaded successfully")
	return nil
}

// TODO - Check if we need retry logic with backoff
// GetImage retrieves an image from MinIO
func (m *MinioClient) GetImage(ctx context.Context, objectName string) (io.ReadCloser, error) {
	reqLogger := logger.FromContext(ctx).With().Str("component", "minio-client").Logger()

	reqLogger.Debug().Str("object", objectName).Msg("Starting image retrieval")

	obj, err := m.client.GetObject(ctx, m.bucketName, objectName, minioLib.GetObjectOptions{})
	if err != nil {
		reqLogger.Error().Err(err).Str("object", objectName).Msg("Error getting image")
		return nil, fmt.Errorf("error getting image: %w", err)
	}

	reqLogger.Debug().Str("object", objectName).Msg("Image retrieved successfully")
	return obj, nil
}

// DeleteImage deletes an image from MinIO
func (m *MinioClient) DeleteImage(ctx context.Context, objectName string) error {
	reqLogger := logger.FromContext(ctx).With().Str("component", "minio-client").Logger()
	err := m.client.RemoveObject(ctx, m.bucketName, objectName, minioLib.RemoveObjectOptions{})
	if err != nil {
		reqLogger.Error().Err(err).Str("object", objectName).Msg("Error deleting image")
		return fmt.Errorf("error deleting image: %w", err)
	}

	reqLogger.Debug().Str("object", objectName).Msg("Image deleted successfully")
	return nil
}

// GetImageURL generates a pre-signed URL for an image in MinIO
func (m *MinioClient) GetImageURL(ctx context.Context, objectName string, expires time.Duration) (string, error) {
	reqLogger := logger.FromContext(ctx).With().Str("component", "minio-client").Logger()

	reqLogger.Debug().Str("object", objectName).Msg("Generating pre-signed URL")
	url, err := m.client.PresignedGetObject(ctx, m.bucketName, objectName, expires, nil)
	if err != nil {
		reqLogger.Error().Err(err).Str("object", objectName).Msg("Error generating pre-signed URL")
		return "", fmt.Errorf("error generating pre-signed URL: %w", err)
	}

	reqLogger.Debug().Str("object", objectName).Msg("Pre-signed URL generated successfully")
	return url.String(), nil
}

// GenerateObjectName generates a unique object name
func (m *MinioClient) GenerateObjectName(id uuid.UUID, fileName string) string {
	ext := path.Ext(fileName)
	base := strings.TrimSuffix(path.Base(fileName), ext)
	sanitizedBase := sanitizeFileName(base)
	return fmt.Sprintf("%s/%s%s", id.String(), sanitizedBase, ext)
}

// Close closes the MinIO client connection
func (m *MinioClient) Close() error {
	return nil
}

// sanitizeFileName sanitizes a file name for storage
func sanitizeFileName(fileName string) string {
	// Replace special characters with underscores
	fileName = strings.ReplaceAll(fileName, " ", "_")

	// Remove any special characters
	fileName = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			return r
		}
		return -1
	}, fileName)

	return fileName
}
