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
	"github.com/rs/zerolog"
)

type MinioClient struct {
	client     *minioLib.Client
	bucketName string
	logger     zerolog.Logger
	config     *config.MinIOConfig
}

func NewClient(cfg *config.MinIOConfig) (minio.Client, error) {
	log := logger.GetLogger("minio-client")

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
		logger:     log,
		config:     cfg,
	}

	exists, err := client.BucketExists(context.Background(), cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("error checking if bucket exists: %w", err)
	}

	if !exists {
		err = client.MakeBucket(context.Background(), cfg.Bucket, minioLib.MakeBucketOptions{Region: cfg.Location})
		if err != nil {
			return nil, fmt.Errorf("error creating bucket: %w", err)
		}
		log.Info().Str("bucket", cfg.Bucket).Msg("Bucket created")
	} else {
		log.Info().Str("bucket", cfg.Bucket).Msg("Bucket already exists")
	}

	return mc, nil
}

// TODO - Check if we need retry logic with backoff
// UploadImage uploads an image to MinIO
func (m *MinioClient) UploadImage(ctx context.Context, reader io.Reader, objectName string, contentType string) error {
	_, err := m.client.PutObject(ctx, m.bucketName, objectName, reader, -1,
		minioLib.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("error uploading image: %w", err)
	}

	m.logger.Debug().Str("object", objectName).Msg("Image uploaded successfully")
	return nil
}

// TODO - Check if we need retry logic with backoff
// GetImage retrieves an image from MinIO
func (m *MinioClient) GetImage(ctx context.Context, objectName string) (io.ReadCloser, error) {
	obj, err := m.client.GetObject(ctx, m.bucketName, objectName, minioLib.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting image: %w", err)
	}

	m.logger.Debug().Str("object", objectName).Msg("Image retrieved successfully")
	return obj, nil
}

// DeleteImage deletes an image from MinIO
func (m *MinioClient) DeleteImage(ctx context.Context, objectName string) error {
	err := m.client.RemoveObject(ctx, m.bucketName, objectName, minioLib.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("error deleting image: %w", err)
	}

	m.logger.Debug().Str("object", objectName).Msg("Image deleted successfully")
	return nil
}

// GetImageURL generates a pre-signed URL for an image in MinIO
func (m *MinioClient) GetImageURL(ctx context.Context, objectName string, expires time.Duration) (string, error) {
	url, err := m.client.PresignedGetObject(ctx, m.bucketName, objectName, expires, nil)
	if err != nil {
		return "", fmt.Errorf("error generating pre-signed URL: %w", err)
	}

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
