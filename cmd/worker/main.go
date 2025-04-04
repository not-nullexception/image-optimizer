package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"

	"github.com/not-nullexception/image-optimizer/config"
	"github.com/not-nullexception/image-optimizer/internal/db/postgres"
	"github.com/not-nullexception/image-optimizer/internal/logger"
	"github.com/not-nullexception/image-optimizer/internal/minio/minio"
	"github.com/not-nullexception/image-optimizer/internal/queue/rabbitmq"
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

	// Create worker
	w := worker.New(repo, minioClient, queueClient, cfg)

	// Start worker
	if err := w.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to start worker")
	}

	// Set up signal handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Wait for interrupt signal
	<-quit
	log.Info().Msg("Shutting down worker...")

	// Cancel the context to signal all services to shut down
	cancel()

	// Wait for worker to finish
	w.Stop()

	log.Info().Msg("Worker stopped")
}
