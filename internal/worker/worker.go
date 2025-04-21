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
	"github.com/not-nullexception/image-optimizer/internal/metrics"
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
	baseLogger  zerolog.Logger
	config      *config.Config
	sem         chan struct{} // Semafor to limit concurrent tasks
	wg          sync.WaitGroup
}

// New create a new worker instance.
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
		baseLogger:  logger.GetLogger("worker"), // Base logger for the worker
		config:      config,
		sem:         make(chan struct{}, config.Worker.MaxWorkers),
	}
}

// Start starts the worker process.
func (w *Worker) Start(ctx context.Context) error {
	w.baseLogger.Info().Int("max_concurrent_tasks", w.config.Worker.MaxWorkers).Msg("Starting worker process")

	err := w.queueClient.Consume(ctx, w.processTask)
	if err != nil {
		w.baseLogger.Error().Err(err).Msg("Worker failed to start consuming messages")
		return fmt.Errorf("error consuming messages: %w", err)
	}
	w.baseLogger.Info().Msg("Worker started and consuming tasks")
	return nil
}

// Stop wait for all tasks to complete and then stops the worker.
func (w *Worker) Stop() {
	w.baseLogger.Info().Msg("Waiting for active worker tasks to complete...")
	close(w.sem) // close the semaphore channel to signal shutdown
	w.wg.Wait()  // wait for all tasks to finish
	w.baseLogger.Info().Msg("All active tasks completed. Worker stopped.")
}

// processTask called by the queue client for each task.
func (w *Worker) processTask(ctx context.Context, task rabbitmq.Task) error {
	w.wg.Add(1)
	defer w.wg.Done()

	taskLogger := logger.FromContext(ctx).With().
		Str("task_id", task.ID).
		Str("task_type", string(task.Type)).
		Logger()
	ctx = logger.ToContext(ctx, taskLogger) // update context with task logger

	taskLogger.Debug().Msg("Acquiring semaphore slot...")
	// check if we can acquire a semaphore slot
	select {
	case w.sem <- struct{}{}:
		// Acquired a slot
		taskLogger.Debug().Msg("Semaphore slot acquired.")
		defer func() {
			<-w.sem // release the slot
			taskLogger.Debug().Msg("Semaphore slot released.")
		}()
	case <-ctx.Done():
		taskLogger.Warn().Msg("Context cancelled while waiting for semaphore slot; task not processed.")
		return ctx.Err()
	}

	// if we reach here, we have acquired a semaphore slot
	taskLogger.Info().Msg("Starting task processing")

	var err error
	switch task.Type {
	case rabbitmq.TaskTypeResizeImage:
		err = w.processImageResize(ctx, task) // pass the context
	default:
		err = fmt.Errorf("unknown task type: %s", string(task.Type))
		taskLogger.Error().Err(err).Msg("Cannot process unknown task type")
	}

	if err != nil {
		taskLogger.Error().Err(err).Msg("Task processing failed")
		return err // return the error to Nack in RabbitMQ
	}

	taskLogger.Info().Msg("Task processing completed successfully")
	return nil // return nil to Ack in RabbitMQ
}

// processImageResize processes the image resize task.
func (w *Worker) processImageResize(ctx context.Context, task rabbitmq.Task) error {
	startTime := time.Now()

	taskLogger := logger.FromContext(ctx).With().Str("component", "worker-image-processor").Logger()

	var imageID string
	var originalPath, filename string
	var configData map[string]interface{}
	var ok bool

	if imageID, ok = task.Data["image_id"].(string); !ok {
		taskLogger.Error().Msg("Missing or invalid image_id in task data")
		return fmt.Errorf("missing or invalid image_id in task data")
	}
	if originalPath, ok = task.Data["original_path"].(string); !ok {
		taskLogger.Error().Str("image_id", imageID).Msg("Missing or invalid original_path in task data")
		return fmt.Errorf("missing or invalid original_path in task data")
	}
	if filename, ok = task.Data["filename"].(string); !ok {
		taskLogger.Error().Str("image_id", imageID).Msg("Missing or invalid filename in task data")
		return fmt.Errorf("missing or invalid filename in task data")
	}
	if configData, ok = task.Data["config"].(map[string]interface{}); !ok {
		taskLogger.Error().Str("image_id", imageID).Msg("Missing or invalid config in task data")
		return fmt.Errorf("missing or invalid config in task data")
	}

	id, err := uuid.Parse(imageID)
	if err != nil {
		taskLogger.Error().Err(err).Str("provided_id", imageID).Msg("Invalid image ID format")
		return fmt.Errorf("invalid image ID format '%s': %w", imageID, err)
	}
	// Add image_id to the logger context
	taskLogger = taskLogger.With().Str("image_id", imageID).Logger()
	ctx = logger.ToContext(ctx, taskLogger) // Atualiza contexto

	taskLogger.Info().Msg("Processing image resize task")

	// update image status to processing in DB
	taskLogger.Debug().Msg("Updating image status to processing in DB")
	err = w.repo.UpdateImageStatus(ctx, id, models.StatusProcessing, "") // Passa o ctx
	if err != nil {
		taskLogger.Error().Err(err).Msg("Failed to update image status to processing")
		metrics.RecordProcessingTime(ctx, "db_status_update_error", startTime) // Registra métrica de falha
		return fmt.Errorf("error updating image status before processing: %w", err)
	}

	// parse configs and set defaults
	// TODO: Move default values to config file
	const defaultMaxWidth = 1200
	const defaultMaxHeight = 1200
	const defaultQuality = 85
	const defaultOptimizeStorage = true

	var processorConfig imageprocessor.Config

	// Parse config data from task
	if mwF, ok := configData["max_width"].(float64); ok { // JSON unmarshal can return float64
		processorConfig.MaxWidth = int(mwF)
	} else {
		processorConfig.MaxWidth = defaultMaxWidth
	}

	if mhF, ok := configData["max_height"].(float64); ok {
		processorConfig.MaxHeight = int(mhF)
	} else {
		processorConfig.MaxHeight = defaultMaxHeight
	}

	if qF, ok := configData["quality"].(float64); ok {
		processorConfig.Quality = int(qF)
	} else {
		processorConfig.Quality = defaultQuality
	}

	if opt, ok := configData["optimize_storage"].(bool); ok {
		processorConfig.OptimizeStorage = opt
	} else {
		processorConfig.OptimizeStorage = defaultOptimizeStorage
	}

	// Apply default values if not set
	if processorConfig.MaxWidth <= 0 {
		processorConfig.MaxWidth = defaultMaxWidth
	}
	if processorConfig.MaxHeight <= 0 {
		processorConfig.MaxHeight = defaultMaxHeight
	}
	if processorConfig.Quality <= 0 || processorConfig.Quality > 100 {
		processorConfig.Quality = defaultQuality
	}

	taskLogger.Info().
		Int("max_width", processorConfig.MaxWidth).
		Int("max_height", processorConfig.MaxHeight).
		Int("quality", processorConfig.Quality).
		Bool("optimize_storage", processorConfig.OptimizeStorage).
		Msg("Effective image processing configuration")

	// Get original image size from DB for metrics
	taskLogger.Debug().Msg("Fetching original image size from DB for metrics")
	imgData, err := w.repo.GetImageByID(ctx, id) // Passa o ctx
	if err != nil {
		taskLogger.Warn().Err(err).Msg("Could not fetch image data from DB to get original size for metrics")
		imgData = nil // Set to nil to avoid using it later
	}

	// Process the image
	taskLogger.Debug().Msg("Calling image processor")
	result, err := w.processor.ProcessImage(ctx, id, originalPath, filename, processorConfig)
	if err != nil {
		errMsg := fmt.Sprintf("error processing image: %s", err.Error())
		taskLogger.Error().Err(err).Msg("Image processing failed")

		updateErr := w.repo.UpdateImageStatus(ctx, id, models.StatusFailed, errMsg)
		if updateErr != nil {
			taskLogger.Error().Err(updateErr).Msg("Also failed to update image status to failed after processing error")
		}
		metrics.RecordProcessingTime(ctx, "processing_error", startTime) // register failure metric
		return err
	}

	// Update image status to processed in DB
	taskLogger.Debug().Msg("Updating image record with optimized data in DB")
	err = w.repo.UpdateImageOptimized(
		ctx,
		id,
		result.OptimizedPath,
		result.OptimizedSize,
		result.OptimizedWidth,
		result.OptimizedHeight,
	)
	if err != nil {
		errMsg := fmt.Sprintf("error updating image record after successful processing: %s", err.Error())
		taskLogger.Error().Err(err).Msg("Failed to update image record in DB")
		updateErr := w.repo.UpdateImageStatus(ctx, id, models.StatusFailed, errMsg)
		if updateErr != nil {
			taskLogger.Error().Err(updateErr).Msg("Also failed to update image status to failed after DB update error")
		}
		metrics.RecordProcessingTime(ctx, "db_update_error", startTime) // register failure metric
		return err
	}

	// Metric for processing time success
	metrics.RecordProcessingTime(ctx, "success", startTime)

	// Only record size reduction if we have original image data
	if imgData != nil {
		metrics.RecordSizeReduction(ctx, imgData.OriginalSize, result.OptimizedSize)
	} else {
		taskLogger.Warn().Msg("Skipping size reduction metric: original image data could not be fetched earlier.")
	}

	taskLogger.Info().
		Str("optimized_path", result.OptimizedPath).
		Int64("optimized_size", result.OptimizedSize).
		Int("optimized_width", result.OptimizedWidth).
		Int("optimized_height", result.OptimizedHeight).
		Msg("Image processed and record updated successfully")

	return nil
}
