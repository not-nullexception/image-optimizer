package router

import (
	"github.com/gin-gonic/gin"
	"github.com/not-nullexception/image-optimizer/config"
	"github.com/not-nullexception/image-optimizer/internal/api/handlers"
	"github.com/not-nullexception/image-optimizer/internal/api/middleware"
	"github.com/not-nullexception/image-optimizer/internal/db"
	"github.com/not-nullexception/image-optimizer/internal/minio"
	rabbitmq "github.com/not-nullexception/image-optimizer/internal/queue"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func Setup(
	cfg *config.Config,
	repository db.Repository,
	minioClient minio.Client,
	queueClient rabbitmq.Client,
) *gin.Engine {
	// Set Gin mode
	gin.SetMode(cfg.Server.Mode)

	// Create router
	r := gin.New()

	// Apply middleware
	r.Use(middleware.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())

	// Apply metrics middleware if enabled
	if cfg.Metrics.Enabled {
		r.Use(middleware.Metrics())
	}

	// Apply tracing middleware if enabled
	if cfg.Tracing.Enabled {
		r.Use(otelgin.Middleware(cfg.Tracing.ServiceName))
	}

	// Create handlers
	imageHandler := handlers.NewImageHandler(repository, minioClient, queueClient, cfg)
	healthHandler := handlers.NewHealthHandler(repository)

	// Health check
	r.GET("/health", healthHandler.Check)

	// Metrics endpoint
	if cfg.Metrics.Enabled {
		r.GET(cfg.Observability.MetricsEndpoint, gin.WrapH(promhttp.Handler()))
	}

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
