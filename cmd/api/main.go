package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/not-nullexception/image-optimizer/config"
	"github.com/not-nullexception/image-optimizer/internal/api/router"
	"github.com/not-nullexception/image-optimizer/internal/db/postgres"
	"github.com/not-nullexception/image-optimizer/internal/logger"
	"github.com/not-nullexception/image-optimizer/internal/minio/minio"
	"github.com/not-nullexception/image-optimizer/internal/queue/rabbitmq"
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

	// Log the configuration for debugging (make sure to not log sensitive data in production)
	// log.Info().Interface("config", cfg).Msg("Configuration loaded")

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

	// Setup router
	r := router.Setup(cfg, repo, minioClient, queueClient)

	// Configure HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start HTTP server in a goroutine
	go func() {
		log.Info().Str("address", server.Addr).Msg("Starting API server")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("API server failed")
		}
	}()

	// Set up signal handling for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Wait for interruption signal
	<-quit
	log.Info().Msg("Shutting down API server...")

	// Cancel the context to signal all services to shut down
	cancel()

	// Create a deadline for the shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Shut down the server
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatal().Err(err).Msg("API server forced to shutdown")
	}

	log.Info().Msg("API server stopped")
}
