package image

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"path/filepath"

	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	"github.com/not-nullexception/image-optimizer/internal/logger"
	"github.com/not-nullexception/image-optimizer/internal/minio"
	"github.com/rs/zerolog"
)

type Processor struct {
	minioClient minio.Client
	logger      zerolog.Logger
}

type ProcessingResult struct {
	OptimizedPath   string
	OptimizedSize   int64
	OptimizedWidth  int
	OptimizedHeight int
}

type Config struct {
	MaxWidth        int
	MaxHeight       int
	Quality         int
	OptimizeStorage bool
}

func New(minioClient minio.Client) *Processor {
	return &Processor{
		minioClient: minioClient,
		logger:      logger.GetLogger("image-processor"),
	}
}

// ProcessImage processes an image from MinIO
func (p *Processor) ProcessImage(ctx context.Context, imageID uuid.UUID, originalPath string, filename string, config Config) (*ProcessingResult, error) {
	p.logger.Info().
		Str("image_id", imageID.String()).
		Str("path", originalPath).
		Msg("Processing image")

	// Get the image from MinIO
	reader, err := p.minioClient.GetImage(ctx, originalPath)
	if err != nil {
		return nil, fmt.Errorf("error getting image from MinIO: %w", err)
	}
	defer reader.Close()

	// Read the entire image into memory
	imgData, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("error reading image data: %w", err)
	}

	// Decode the image
	img, format, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return nil, fmt.Errorf("error decoding image: %w", err)
	}

	// Get original dimensions
	bounds := img.Bounds()
	originalWidth := bounds.Dx()
	originalHeight := bounds.Dy()

	p.logger.Debug().
		Str("image_id", imageID.String()).
		Str("format", format).
		Int("original_width", originalWidth).
		Int("original_height", originalHeight).
		Int("original_size", len(imgData)).
		Msg("Image details")

	// Calculate new dimensions while maintaining aspect ratio
	var newWidth, newHeight int
	if config.MaxWidth > 0 && config.MaxHeight > 0 {
		// Calculate scaling factors
		widthFactor := float64(config.MaxWidth) / float64(originalWidth)
		heightFactor := float64(config.MaxHeight) / float64(originalHeight)

		// Use the smaller factor to ensure the image fits within the maximum dimensions
		scaleFactor := math.Min(widthFactor, heightFactor)

		// Only resize if the image is larger than the target dimensions
		if scaleFactor < 1.0 {
			newWidth = int(float64(originalWidth) * scaleFactor)
			newHeight = int(float64(originalHeight) * scaleFactor)
		} else {
			// If the image is already smaller than the target dimensions, keep original size
			newWidth = originalWidth
			newHeight = originalHeight
		}
	} else {
		// If no maximum dimensions are specified, keep original size
		newWidth = originalWidth
		newHeight = originalHeight
	}

	// Resize the image if needed
	var resizedImg image.Image
	if newWidth != originalWidth || newHeight != originalHeight {
		resizedImg = imaging.Resize(img, newWidth, newHeight, imaging.Lanczos)
		p.logger.Debug().
			Str("image_id", imageID.String()).
			Int("new_width", newWidth).
			Int("new_height", newHeight).
			Msg("Image resized")
	} else {
		resizedImg = img
		p.logger.Debug().
			Str("image_id", imageID.String()).
			Msg("No resizing needed")
	}

	// Create a buffer to hold the processed image
	var buf bytes.Buffer

	// Set quality and encode the image based on format
	var processingErr error
	var contentType string

	// Generate unique path for the processed image
	ext := filepath.Ext(filename)
	optimizedPath := fmt.Sprintf("%s/optimized%s", imageID.String(), ext)

	switch format {
	case "jpeg":
		contentType = "image/jpeg"
		processingErr = jpeg.Encode(&buf, resizedImg, &jpeg.Options{
			Quality: config.Quality,
		})
	case "png":
		contentType = "image/png"
		encoder := png.Encoder{
			CompressionLevel: png.BestCompression,
		}
		processingErr = encoder.Encode(&buf, resizedImg)
	default:
		return nil, fmt.Errorf("unsupported image format: %s", format)
	}

	if processingErr != nil {
		return nil, fmt.Errorf("error encoding processed image: %w", processingErr)
	}

	// Get the processed image data
	processedImgData := buf.Bytes()

	// Only upload if the processed image is smaller than the original or if we forced resizing
	if len(processedImgData) < len(imgData) || newWidth != originalWidth || newHeight != originalHeight || config.OptimizeStorage {
		// Upload the processed image to MinIO
		err = p.minioClient.UploadImage(ctx, bytes.NewReader(processedImgData), optimizedPath, contentType)
		if err != nil {
			return nil, fmt.Errorf("error uploading processed image: %w", err)
		}

		p.logger.Info().
			Str("image_id", imageID.String()).
			Int("original_size", len(imgData)).
			Int("processed_size", len(processedImgData)).
			Float64("reduction_percentage", (1-float64(len(processedImgData))/float64(len(imgData)))*100).
			Msg("Image processed and uploaded")

		return &ProcessingResult{
			OptimizedPath:   optimizedPath,
			OptimizedSize:   int64(len(processedImgData)),
			OptimizedWidth:  newWidth,
			OptimizedHeight: newHeight,
		}, nil
	}

	// If no optimization was achieved and we're not forcing optimization, use the original
	p.logger.Info().
		Str("image_id", imageID.String()).
		Msg("No optimization achieved, using original image")

	return &ProcessingResult{
		OptimizedPath:   originalPath,
		OptimizedSize:   int64(len(imgData)),
		OptimizedWidth:  originalWidth,
		OptimizedHeight: originalHeight,
	}, nil
}

// ValidateImage checks if an image is valid and returns its dimensions and size
func (p *Processor) ValidateImage(reader io.Reader) (int, int, int64, string, error) {
	// Read the entire image into memory
	imgData, err := io.ReadAll(reader)
	if err != nil {
		return 0, 0, 0, "", fmt.Errorf("error reading image data: %w", err)
	}

	// Decode the image
	img, format, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return 0, 0, 0, "", fmt.Errorf("error decoding image: %w", err)
	}

	// Check if format is supported
	if format != "jpeg" && format != "png" {
		return 0, 0, 0, "", fmt.Errorf("unsupported image format: %s", format)
	}

	// Get dimensions
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	size := int64(len(imgData))

	return width, height, size, format, nil
}
