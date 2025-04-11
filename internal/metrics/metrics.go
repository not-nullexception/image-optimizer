package metrics

import (
	"context"
	"time"

	"github.com/not-nullexception/image-optimizer/internal/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestsTotal counts the number of HTTP requests received
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "image_optimizer_requests_total",
			Help: "The total number of HTTP requests processed by the API",
		},
		[]string{"method", "endpoint", "status"},
	)

	// RequestDuration measures the duration of HTTP requests
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "image_optimizer_request_duration_seconds",
			Help:    "The duration of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	// ProcessingTotal counts total processed images
	ProcessingTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "image_optimizer_processing_total",
			Help: "The total number of processed images",
		},
		[]string{"status"},
	)

	// ProcessingDuration measures the duration of image processing
	ProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "image_optimizer_processing_duration_seconds",
			Help:    "The duration of image processing in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // From 100ms to ~100s
		},
		[]string{"status"},
	)

	// ImageSizeReduction measures the image size reduction percentage
	ImageSizeReduction = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "image_optimizer_size_reduction_percentage",
			Help:    "The percentage of size reduction for processed images",
			Buckets: prometheus.LinearBuckets(0, 10, 11), // 0% to 100% in 10% increments
		},
	)

	// QueueDepth gauges the current depth of the processing queue
	QueueDepth = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "image_optimizer_queue_depth",
			Help: "The current depth of the processing queue",
		},
	)

	// WorkerUtilization gauges the percentage of workers currently in use
	WorkerUtilization = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "image_optimizer_worker_utilization",
			Help: "The percentage of workers currently processing tasks",
		},
	)

	// StorageUsage gauges the current storage usage in bytes
	StorageUsage = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "image_optimizer_storage_usage_bytes",
			Help: "The current storage usage in bytes",
		},
	)

	// DBConnections gauges the number of active database connections
	DBConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "image_optimizer_db_connections",
			Help: "The number of active database connections",
		},
	)
)

// RecordProcessingTime records the time taken to process an image
func RecordProcessingTime(ctx context.Context, status string, startTime time.Time) {
	duration := time.Since(startTime).Seconds()
	ProcessingDuration.WithLabelValues(status).Observe(duration)
	ProcessingTotal.WithLabelValues(status).Inc()

	reqLogger := logger.FromContext(ctx)

	reqLogger.Debug().
		Str("status", status).
		Float64("duration_seconds", duration).
		Msg("Recorded image processing time")
}

// RecordSizeReduction records the percentage of size reduction
func RecordSizeReduction(ctx context.Context, originalSize, optimizedSize int64) {
	if originalSize <= 0 {
		return
	}

	percentage := (1 - (float64(optimizedSize) / float64(originalSize))) * 100
	ImageSizeReduction.Observe(percentage)

	reqLogger := logger.FromContext(ctx)

	reqLogger.Debug().
		Int64("original_size", originalSize).
		Int64("optimized_size", optimizedSize).
		Float64("reduction_percentage", percentage).
		Msg("Recorded image size reduction")
}

// UpdateQueueDepth updates the queue depth metric
func UpdateQueueDepth(depth int) {
	QueueDepth.Set(float64(depth))
}

// UpdateWorkerUtilization updates the worker utilization metric
func UpdateWorkerUtilization(active, total int) {
	if total <= 0 {
		return
	}

	percentage := (float64(active) / float64(total)) * 100
	WorkerUtilization.Set(percentage)
}

// UpdateStorageUsage updates the storage usage metric
func UpdateStorageUsage(usageBytes int64) {
	StorageUsage.Set(float64(usageBytes))
}

// UpdateDBConnections updates the database connections metric
func UpdateDBConnections(connections int) {
	DBConnections.Set(float64(connections))
}

// Init initializes metrics collection
func Init() {
	logger := logger.GetLogger("metrics")
	logger.Info().Msg("Metrics collection initialized")
}
