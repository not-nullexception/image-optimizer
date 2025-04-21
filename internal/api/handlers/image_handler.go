package handlers

import (
	"context"
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
		config:      config,
	}
}

// UploadImage handles image upload requests
func (h *ImageHandler) UploadImage(c *gin.Context) {
	// TODO - Improve input validation

	reqLogger := logger.FromContext(c.Request.Context())
	reqLogger.Info().Msg("Received image upload request")

	// Get file from request
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get image from request"})
		return
	}
	defer file.Close()

	// Check file size
	if header.Size > 10*1024*1024 { // 10 MB
		reqLogger.Error().Str("filename", header.Filename).Int64("size", header.Size).Msg("File too large")
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large, max 10MB"})
		return
	}

	// Validate file type
	ext := filepath.Ext(header.Filename)
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		reqLogger.Error().Str("filename", header.Filename).Str("extension", ext).Msg("Unsupported file format")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported file format, only JPG and PNG are supported"})
		return
	}

	// Validate MIME type
	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		reqLogger.Error().Err(err).Str("filename", header.Filename).Msg("Failed to read file for MIME type validation")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read file for MIME type validation"})
		return
	}
	file.Seek(0, 0) // Reset file position after reading

	mimeType := http.DetectContentType(buffer)
	if mimeType != "image/jpeg" && mimeType != "image/png" {
		reqLogger.Error().Str("filename", header.Filename).Str("provided_mime", mimeType).Msg("Unsupported MIME type")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported MIME type, only image/jpeg and image/png are supported"})
		return
	}

	// Validate the image and get dimensions
	width, height, size, format, err := h.processor.ValidateImage(c.Request.Context(), file)
	if err != nil {
		reqLogger.Error().Err(err).Str("filename", header.Filename).Msg("Invalid image")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image: " + err.Error()})
		return
	}

	// Reset file position for uploading
	file.Seek(0, 0)

	// Generate ID for the image
	imageUUID := uuid.New()
	reqLogger.Info().Str("image_id", imageUUID.String()).Str("filename", header.Filename).Msg("Generated unique ID for new image upload")

	objectName := h.minioClient.GenerateObjectName(imageUUID, header.Filename)

	// Upload original image to MinIO
	contentType := "image/jpeg"
	if format == "png" {
		contentType = "image/png"
	}

	err = h.minioClient.UploadImage(c.Request.Context(), file, objectName, contentType)
	if err != nil {
		reqLogger.Error().Err(err).Str("filename", header.Filename).Msg("Failed to upload image to storage")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload image to storage"})
		return
	}

	// Create image record in database
	img := models.NewImageWithID(imageUUID, header.Filename, size, width, height, format, objectName)

	err = h.repo.CreateImage(c.Request.Context(), img)
	if err != nil {
		reqLogger.Error().Err(err).Str("id", imageUUID.String()).Msg("Failed to save image metadata to database")
		cleanupErr := h.minioClient.DeleteImage(context.Background(), objectName)
		if cleanupErr != nil {
			reqLogger.Error().Err(cleanupErr).Str("object_name", objectName).Msg("Failed to cleanup MinIO object after DB error")
		}
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

	if finalConfigMap, ok := task.Data["config"].(map[string]any); ok {
		// Verifique se 'ok' é true antes de tentar acessar o mapa
		// Use zerolog.Dict() para logar os valores finais de forma estruturada
		reqLogger.Debug().Dict("final_task_config", zerolog.Dict().
			Int("max_width", finalConfigMap["max_width"].(int)).   // Faz type assertion para int
			Int("max_height", finalConfigMap["max_height"].(int)). // Assume que os tipos no mapa estão corretos
			Int("quality", finalConfigMap["quality"].(int)).
			Bool("optimize_storage", finalConfigMap["optimize_storage"].(bool)), // Inclui o campo booleano
		).Msg("Applied custom parameters; final task configuration prepared")
	} else {
		// Logue um aviso se, por algum motivo, o mapa de configuração não estiver lá ou for do tipo errado
		reqLogger.Warn().Msg("Could not log final task config: task.Data[\"config\"] is not a map[string]any")
	}

	err = h.queueClient.Publish(c.Request.Context(), task)
	if err != nil {
		reqLogger.Error().Err(err).Str("id", imageUUID.String()).Msg("Failed to queue image for processing")
		// Continue anyway, as we have stored the original image
		// TODO - consider adding a retry mechanism or a dead-letter queue
	}

	reqLogger.Info().Str("id", imageUUID.String()).Msg("Image accepted and queued for processing")

	// Return image ID
	c.JSON(http.StatusAccepted, &models.ImageUploadResponse{
		ID:     imageUUID,
		Status: string(models.StatusPending),
	})
}

// GetImage retrieves information about an image
func (h *ImageHandler) GetImage(c *gin.Context) {
	reqLogger := logger.FromContext(c.Request.Context())

	// Parse the ID from the URL
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image ID"})
		return
	}

	reqLogger.Info().Str("image_id", idStr).Msg("Processing get image request")

	// Get the image from the database
	img, err := h.repo.GetImageByID(c.Request.Context(), id)
	if err != nil {
		reqLogger.Error().Err(err).Str("id", idStr).Msg("Failed to get image")
		c.JSON(http.StatusNotFound, gin.H{"error": "Image not found"})
		return
	}

	// Generate URLs for the image
	var originalURL, optimizedURL string

	// Generate URL for original image
	originalURL, err = h.minioClient.GetImageURL(c.Request.Context(), img.OriginalPath, h.config.MinIO.URLExpiry)
	if err != nil {
		reqLogger.Error().Err(err).Str("id", idStr).Msg("Failed to generate URL for original image")
		// Continue anyway, as we have stored the original image
	}

	// Generate URL for optimized image if available
	if img.Status == models.StatusCompleted && img.OptimizedPath != "" {
		optimizedURL, err = h.minioClient.GetImageURL(c.Request.Context(), img.OptimizedPath, h.config.MinIO.URLExpiry)
		if err != nil {
			reqLogger.Error().Err(err).Str("id", idStr).Msg("Failed to generate URL for optimized image")
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

	reqLogger.Info().Str("image_id", idStr).Str("status", string(img.Status)).Msg("Image retrieved successfully")

	c.JSON(http.StatusOK, response)
}

// ListImages lists all images
func (h *ImageHandler) ListImages(c *gin.Context) {
	reqLogger := logger.FromContext(c.Request.Context())

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

	reqLogger.Info().Int("limit", limit).Int("page", page).Msg("Processing list images request")

	// Calculate offset
	offset := (page - 1) * limit

	// Get images from the database
	images, total, err := h.repo.ListImages(c.Request.Context(), limit, offset)
	if err != nil {
		reqLogger.Error().Err(err).Msg("Failed to list images")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list images"})
		return
	}

	// Create response
	response := &models.ImageListResponse{
		Images: images,
		Total:  total,
	}

	reqLogger.Info().Int("count", len(images)).Int("total_db", total).Msg("Images listed successfully")

	c.JSON(http.StatusOK, response)
}

// DeleteImage deletes an image
func (h *ImageHandler) DeleteImage(c *gin.Context) {
	reqLogger := logger.FromContext(c.Request.Context())

	// Parse the ID from the URL
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image ID"})
		return
	}

	reqLogger.Info().Str("image_id", idStr).Msg("Processing delete image request")

	// Get the image from the database
	img, err := h.repo.GetImageByID(c.Request.Context(), id)
	if err != nil {
		reqLogger.Error().Err(err).Str("id", idStr).Msg("Failed to get image")
		c.JSON(http.StatusNotFound, gin.H{"error": "Image not found"})
		return
	}

	// Delete original image from MinIO
	err = h.minioClient.DeleteImage(c.Request.Context(), img.OriginalPath)
	if err != nil {
		reqLogger.Error().Err(err).Str("id", idStr).Msg("Failed to delete original image from storage")
		// Continue anyway, as we want to clean up the database
		// TODO - consider adding cleanup logic for orphaned images in MinIO
	}

	// Delete optimized image from MinIO if it exists
	if img.OptimizedPath != "" && img.OptimizedPath != img.OriginalPath {
		err = h.minioClient.DeleteImage(c.Request.Context(), img.OptimizedPath)
		if err != nil {
			reqLogger.Error().Err(err).Str("id", idStr).Msg("Failed to delete optimized image from storage")
			// Continue anyway
			// TODO - consider adding cleanup logic for orphaned images in MinIO
		}
	}

	// Delete the image from the database
	err = h.repo.DeleteImage(c.Request.Context(), id)
	if err != nil {
		reqLogger.Error().Err(err).Str("id", idStr).Msg("Failed to delete image from database")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete image"})
		return
	}

	reqLogger.Info().Str("image_id", idStr).Msg("Image deleted successfully")

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
