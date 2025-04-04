package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

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

type Worker struct {
	repo        db.Repository
	minioClient minio.Client
	queueClient rabbitmq.Client
	processor   *imageprocessor.Processor
	logger      zerolog.Logger
	config      *config.Config
	sem         chan struct{} // Semaphore to limit concurrent processing
	wg          sync.WaitGroup
}

func New(
	repo db.Repository,
	minioClient minio.Client,
	queueClient rabbitmq.Client,
	config *config.Config,
) *Worker {
	return &Worker{
		repo:        repo,
		minioClient: minioClient,
		queueClient: queueClient,
		processor:   imageprocessor.New(minioClient),
		logger:      logger.GetLogger("worker"),
		config:      config,
		sem:         make(chan struct{}, config.Worker.MaxWorkers), // Limit concurrent processing
	}
}

// Start starts the worker
func (w *Worker) Start(ctx context.Context) error {
	w.logger.Info().Int("worker_count", w.config.Worker.Count).Msg("Starting worker")

	err := w.queueClient.Consume(ctx, w.processTask)
	if err != nil {
		return fmt.Errorf("error consuming messages: %w", err)
	}

	return nil
}

// Stop stops the worker
func (w *Worker) Stop() {
	w.logger.Info().Msg("Stopping worker")
	w.wg.Wait() // Wait for all tasks to finish
	w.logger.Info().Msg("Worker stopped")
}

// processTask processes a task from the queue
func (w *Worker) processTask(ctx context.Context, task rabbitmq.Task) error {
	w.wg.Add(1)
	defer w.wg.Done()

	// Acquire semaphore
	w.sem <- struct{}{}
	defer func() { <-w.sem }() // Release semaphore when done

	w.logger.Info().
		Str("task_id", task.ID).
		Str("task_type", string(task.Type)).
		Msg("Processing task")

	// Handle different task types
	switch task.Type {
	case rabbitmq.TaskTypeResizeImage:
		return w.processImageResize(ctx, task)
	default:
		return fmt.Errorf("unknown task type: %s", string(task.Type))
	}
}

// processImageResize processes an image resize task
func (w *Worker) processImageResize(ctx context.Context, task rabbitmq.Task) error {
	// Extract data from the task
	imageID, ok := task.Data["image_id"].(string)
	if !ok {
		return fmt.Errorf("missing image_id in task data")
	}

	originalPath, ok := task.Data["original_path"].(string)
	if !ok {
		return fmt.Errorf("missing original_path in task data")
	}

	filename, ok := task.Data["filename"].(string)
	if !ok {
		return fmt.Errorf("missing filename in task data")
	}

	configData, ok := task.Data["config"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing config in task data")
	}

	id, err := uuid.Parse(imageID)
	if err != nil {
		return fmt.Errorf("invalid image ID: %w", err)
	}

	// Update image status to processing
	err = w.repo.UpdateImageStatus(ctx, id, models.StatusProcessing, "")
	if err != nil {
		return fmt.Errorf("error updating image status: %w", err)
	}

	// Parse config
	var processorConfig imageprocessor.Config
	processorConfig.MaxWidth, _ = configData["max_width"].(int)
	processorConfig.MaxHeight, _ = configData["max_height"].(int)
	processorConfig.Quality, _ = configData["quality"].(int)
	processorConfig.OptimizeStorage, _ = configData["optimize_storage"].(bool)

	// Set default values if values are invalid
	if processorConfig.MaxWidth <= 0 {
		processorConfig.MaxWidth = 1200
	}
	if processorConfig.MaxHeight <= 0 {
		processorConfig.MaxHeight = 1200
	}
	if processorConfig.Quality <= 0 || processorConfig.Quality > 100 {
		processorConfig.Quality = 85
	}

	// Process the image
	w.logger.Info().
		Str("image_id", imageID).
		Int("max_width", processorConfig.MaxWidth).
		Int("max_height", processorConfig.MaxHeight).
		Int("quality", processorConfig.Quality).
		Bool("optimize_storage", processorConfig.OptimizeStorage).
		Msg("Processing image with config")

	// Add a small delay to avoid overwhelming the system
	// TODO : Change this to a more sophisticated rate limiting mechanism
	time.Sleep(100 * time.Millisecond)

	result, err := w.processor.ProcessImage(ctx, id, originalPath, filename, processorConfig)
	if err != nil {
		errMsg := fmt.Sprintf("error processing image: %s", err.Error())
		w.logger.Error().Err(err).Str("image_id", imageID).Msg(errMsg)

		// Update image status to failed
		updateErr := w.repo.UpdateImageStatus(ctx, id, models.StatusFailed, errMsg)
		if updateErr != nil {
			w.logger.Error().Err(updateErr).Str("image_id", imageID).Msg("Error updating image status")
		}

		return err
	}

	// Update the image with optimized information
	err = w.repo.UpdateImageOptimized(
		ctx,
		id,
		result.OptimizedPath,
		result.OptimizedSize,
		result.OptimizedWidth,
		result.OptimizedHeight,
	)
	if err != nil {
		errMsg := fmt.Sprintf("error updating image: %s", err.Error())
		w.logger.Error().Err(err).Str("image_id", imageID).Msg(errMsg)

		// Update image status to failed
		updateErr := w.repo.UpdateImageStatus(ctx, id, models.StatusFailed, errMsg)
		if updateErr != nil {
			w.logger.Error().Err(updateErr).Str("image_id", imageID).Msg("Error updating image status")
		}

		return err
	}

	w.logger.Info().
		Str("image_id", imageID).
		Str("optimized_path", result.OptimizedPath).
		Int64("optimized_size", result.OptimizedSize).
		Int("optimized_width", result.OptimizedWidth).
		Int("optimized_height", result.OptimizedHeight).
		Msg("Image processed successfully")

	return nil
}
