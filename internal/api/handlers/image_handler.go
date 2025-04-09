package handlers

import (
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/not-nullexception/image-optimizer/config"
	"github.com/not-nullexception/image-optimizer/internal/db"
	"github.com/not-nullexception/image-optimizer/internal/db/models"
	"github.com/not-nullexception/image-optimizer/internal/logger"
	"github.com/not-nullexception/image-optimizer/internal/minio"
	imageprocessor "github.com/not-nullexception/image-optimizer/internal/processor/image"
	rabbitmq "github.com/not-nullexception/image-optimizer/internal/queue"
	"github.com/rs/zerolog"
)

type ImageHandler struct {
	repo        db.Repository
	minioClient minio.Client
	queueClient rabbitmq.Client
	processor   *imageprocessor.Processor
	logger      zerolog.Logger
	config      *config.Config
}

func NewImageHandler(
	repo db.Repository,
	minioClient minio.Client,
	queueClient rabbitmq.Client,
	config *config.Config,
) *ImageHandler {
	return &ImageHandler{
		repo:        repo,
		minioClient: minioClient,
		queueClient: queueClient,
		processor:   imageprocessor.New(minioClient),
		logger:      logger.GetLogger("image-handler"),
		config:      config,
	}
}

// UploadImage handles image upload requests
func (h *ImageHandler) UploadImage(c *gin.Context) {
	// TODO - Improve input validation

	// Get file from request
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get image from request"})
		return
	}
	defer file.Close()

	// Check file size
	if header.Size > 10*1024*1024 { // 10 MB
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large, max 10MB"})
		return
	}

	// Validate file type
	ext := filepath.Ext(header.Filename)
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported file format, only JPG and PNG are supported"})
		return
	}

	// Validate MIME type
	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read file for MIME type validation"})
		return
	}
	file.Seek(0, 0) // Reset file position after reading

	mimeType := http.DetectContentType(buffer)
	if mimeType != "image/jpeg" && mimeType != "image/png" {
		h.logger.Error().Str("filename", header.Filename).Str("mime_type", mimeType).Msg("Unsupported MIME type")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported MIME type, only image/jpeg and image/png are supported"})
		return
	}

	// Validate the image and get dimensions
	width, height, size, format, err := h.processor.ValidateImage(file)
	if err != nil {
		h.logger.Error().Err(err).Str("filename", header.Filename).Msg("Invalid image")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image: " + err.Error()})
		return
	}

	// Reset file position for uploading
	file.Seek(0, 0)

	// Generate ID for the image
	id := uuid.New()

	objectName := h.minioClient.GenerateObjectName(id, header.Filename)

	// Upload original image to MinIO
	contentType := "image/jpeg"
	if format == "png" {
		contentType = "image/png"
	}

	err = h.minioClient.UploadImage(c.Request.Context(), file, objectName, contentType)
	if err != nil {
		h.logger.Error().Err(err).Str("filename", header.Filename).Msg("Failed to upload image")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload image"})
		return
	}

	// Create image record in database
	img := models.NewImage(header.Filename, size, width, height, format, objectName)

	err = h.repo.CreateImage(c.Request.Context(), img)
	if err != nil {
		h.logger.Error().Err(err).Str("id", id.String()).Msg("Failed to save image metadata")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save image metadata"})
		return
	}

	// Send image to processing queue
	task := rabbitmq.Task{
		ID:   img.ID.String(),
		Type: rabbitmq.TaskTypeResizeImage,
		Data: map[string]any{
			"image_id":      img.ID.String(),
			"original_path": img.OriginalPath,
			"filename":      img.OriginalName,
			"config": map[string]any{
				"max_width":        1200, // Default max width
				"max_height":       1200, // Default max height
				"quality":          85,   // Default JPEG quality
				"optimize_storage": true,
			},
		},
	}

	// Process custom parameters if provided
	if width, err := strconv.Atoi(c.DefaultQuery("max_width", "0")); err == nil && width > 0 {
		task.Data["config"].(map[string]any)["max_width"] = width
	}

	if height, err := strconv.Atoi(c.DefaultQuery("max_height", "0")); err == nil && height > 0 {
		task.Data["config"].(map[string]any)["max_height"] = height
	}

	if quality, err := strconv.Atoi(c.DefaultQuery("quality", "0")); err == nil && quality > 0 {
		task.Data["config"].(map[string]any)["quality"] = quality
	}

	err = h.queueClient.Publish(c.Request.Context(), task)
	if err != nil {
		h.logger.Error().Err(err).Str("id", id.String()).Msg("Failed to queue image for processing")
		// Continue anyway, as we have stored the original image
		// TODO - consider adding a retry mechanism or a dead-letter queue
	}

	// Return image ID
	c.JSON(http.StatusAccepted, &models.ImageUploadResponse{
		ID:     id,
		Status: string(models.StatusPending),
	})
}

// GetImage retrieves information about an image
func (h *ImageHandler) GetImage(c *gin.Context) {
	// Parse the ID from the URL
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image ID"})
		return
	}

	// Get the image from the database
	img, err := h.repo.GetImageByID(c.Request.Context(), id)
	if err != nil {
		h.logger.Error().Err(err).Str("id", idStr).Msg("Failed to get image")
		c.JSON(http.StatusNotFound, gin.H{"error": "Image not found"})
		return
	}

	// Generate URLs for the image
	var originalURL, optimizedURL string

	// Generate URL for original image
	originalURL, err = h.minioClient.GetImageURL(c.Request.Context(), img.OriginalPath, h.config.MinIO.URLExpiry)
	if err != nil {
		h.logger.Error().Err(err).Str("id", idStr).Msg("Failed to generate URL for original image")
		// Continue anyway, as we have stored the original image
	}

	// Generate URL for optimized image if available
	if img.Status == models.StatusCompleted && img.OptimizedPath != "" {
		optimizedURL, err = h.minioClient.GetImageURL(c.Request.Context(), img.OptimizedPath, h.config.MinIO.URLExpiry)
		if err != nil {
			h.logger.Error().Err(err).Str("id", idStr).Msg("Failed to generate URL for optimized image")
			// Continue anyway, as we have stored the original image
		}
	}

	// Calculate size reduction percentage
	var reduction float64
	if img.Status == models.StatusCompleted && img.OptimizedSize > 0 && img.OriginalSize > 0 {
		reduction = (1 - float64(img.OptimizedSize)/float64(img.OriginalSize)) * 100
	}

	// Create response
	response := &models.ImageResponse{
		ID:            img.ID,
		OriginalName:  img.OriginalName,
		Status:        img.Status,
		OriginalURL:   originalURL,
		OptimizedURL:  optimizedURL,
		OriginalSize:  img.OriginalSize,
		OptimizedSize: img.OptimizedSize,
		Reduction:     reduction,
		CreatedAt:     img.CreatedAt,
		UpdatedAt:     img.UpdatedAt,
		Error:         img.Error,
	}

	c.JSON(http.StatusOK, response)
}

// ListImages lists all images
func (h *ImageHandler) ListImages(c *gin.Context) {
	// Parse pagination parameters
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))

	// Validation pagination parameters
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	if page <= 0 {
		page = 1
	}

	// Calculate offset
	offset := (page - 1) * limit

	// Get images from the database
	images, total, err := h.repo.ListImages(c.Request.Context(), limit, offset)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to list images")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list images"})
		return
	}

	// Create response
	response := &models.ImageListResponse{
		Images: images,
		Total:  total,
	}

	c.JSON(http.StatusOK, response)
}

// DeleteImage deletes an image
func (h *ImageHandler) DeleteImage(c *gin.Context) {
	// Parse the ID from the URL
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image ID"})
		return
	}

	// Get the image from the database
	img, err := h.repo.GetImageByID(c.Request.Context(), id)
	if err != nil {
		h.logger.Error().Err(err).Str("id", idStr).Msg("Failed to get image")
		c.JSON(http.StatusNotFound, gin.H{"error": "Image not found"})
		return
	}

	// Delete original image from MinIO
	err = h.minioClient.DeleteImage(c.Request.Context(), img.OriginalPath)
	if err != nil {
		h.logger.Error().Err(err).Str("id", idStr).Msg("Failed to delete original image from storage")
		// Continue anyway, as we want to clean up the database
		// TODO - consider adding cleanup logic for orphaned images in MinIO
	}

	// Delete optimized image from MinIO if it exists
	if img.OptimizedPath != "" && img.OptimizedPath != img.OriginalPath {
		err = h.minioClient.DeleteImage(c.Request.Context(), img.OptimizedPath)
		if err != nil {
			h.logger.Error().Err(err).Str("id", idStr).Msg("Failed to delete optimized image from storage")
			// Continue anyway
			// TODO - consider adding cleanup logic for orphaned images in MinIO
		}
	}

	// Delete the image from the database
	err = h.repo.DeleteImage(c.Request.Context(), id)
	if err != nil {
		h.logger.Error().Err(err).Str("id", idStr).Msg("Failed to delete image from database")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete image"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
