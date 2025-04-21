// Arquivo: cmd/worker/main.go
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"

	"github.com/not-nullexception/image-optimizer/config"
	"github.com/not-nullexception/image-optimizer/internal/db/postgres"
	"github.com/not-nullexception/image-optimizer/internal/logger"
	"github.com/not-nullexception/image-optimizer/internal/metrics"
	"github.com/not-nullexception/image-optimizer/internal/minio/minio"
	"github.com/not-nullexception/image-optimizer/internal/queue/rabbitmq"
	"github.com/not-nullexception/image-optimizer/internal/tracing"
	"github.com/not-nullexception/image-optimizer/internal/worker"
)

func main() {
	// Create a context that will be canceled on shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Setup logger
	logger.Setup(&cfg.Log)

	if cfg.Tracing.Enabled {
		traceCfg := tracing.TracingConfig{
			ServiceName:    cfg.Tracing.ServiceName + "-worker",
			ServiceVersion: cfg.Tracing.ServiceVersion,
			Environment:    cfg.Tracing.Environment,
			OTLPEndpoint:   cfg.Tracing.OTLPEndpoint,
			Enabled:        cfg.Tracing.Enabled,
		}
		tracerShutdown, err := tracing.Init(ctx, traceCfg)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize tracing")
		}
		defer tracerShutdown() // shutdown tracer on exit
	}

	// Initialize metrics if enabled
	if cfg.Metrics.Enabled {
		metrics.Init()
	}

	// Create database repository
	repo, err := postgres.NewRepository(ctx, &cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create database repository")
	}
	defer repo.Close()

	// Create MinIO client
	minioClient, err := minio.NewClient(&cfg.MinIO)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create MinIO client")
	}
	defer minioClient.Close()

	// Create RabbitMQ client
	queueClient, err := rabbitmq.NewClient(&cfg.RabbitMQ)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create RabbitMQ client")
	}
	defer queueClient.Close()

	// Start metrics server if enabled
	var metricsServer *http.Server
	if cfg.Metrics.Enabled {
		metricsAddr := fmt.Sprintf(":%d", cfg.Worker.MetricsPort)
		metricsServer = startMetricsServer(metricsAddr)
		log.Info().Str("address", metricsAddr).Msg("Starting metrics server for worker")
	}

	// Create worker
	w := worker.New(repo, minioClient, queueClient, cfg)

	// Start worker
	if err := w.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to start worker")
	}

	// Signal handling for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit // wait for shutdown signal

	log.Info().Msg("Shutting down worker...")

	// cancel the context to stop the worker
	cancel()

	// create a new context for shutdown with a timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second) // Aumentado ligeiramente
	defer shutdownCancel()

	// stop the worker
	w.Stop() // call the Stop method to stop the worker gracefully

	// Stop the metrics server if it was started
	if metricsServer != nil {
		log.Info().Msg("Shutting down metrics server...")
		if err := metricsServer.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("Metrics server shutdown failed")
		} else {
			log.Info().Msg("Metrics server stopped")
		}
	}

	log.Info().Msg("Worker stopped gracefully")
}

// startMetricsServer starts the metrics server for the worker
func startMetricsServer(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler()) // Prometheus metrics endpoint

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second, // Short read timeout for metrics
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// Start the server in a goroutine to avoid blocking
	go func() {
		log.Debug().Str("address", addr).Msg("Metrics server ListenAndServe starting")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Str("address", addr).Msg("Metrics server ListenAndServe failed")
		}
	}()

	return server
}
