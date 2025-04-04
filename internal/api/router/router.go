package router

import (
	"github.com/gin-gonic/gin"
	"github.com/not-nullexception/image-optimizer/config"
	"github.com/not-nullexception/image-optimizer/internal/api/handlers"
	"github.com/not-nullexception/image-optimizer/internal/api/middleware"
	"github.com/not-nullexception/image-optimizer/internal/db"
	"github.com/not-nullexception/image-optimizer/internal/minio"
	rabbitmq "github.com/not-nullexception/image-optimizer/internal/queue"
)

func Setup(
	cfg *config.Config,
	repository db.Repository,
	minioClient minio.Client,
	queueClient rabbitmq.Client,
) *gin.Engine {
	// Set gin mode
	gin.SetMode(cfg.Server.Mode)

	r := gin.New()

	// Apply middlewares
	r.Use(middleware.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())

	// Create handlers
	imageHandler := handlers.NewImageHandler(repository, minioClient, queueClient, cfg)
	healthHandler := handlers.NewHealthHandler(repository)

	// Health check
	r.GET("/health", healthHandler.Check)

	// API routes
	api := r.Group("/api")
	{
		// Image routes
		images := api.Group("/images")
		{
			images.POST("", imageHandler.UploadImage)
			images.GET("", imageHandler.ListImages)
			images.GET("/:id", imageHandler.GetImage)
			images.DELETE("/:id", imageHandler.DeleteImage)
		}
	}

	return r
}
