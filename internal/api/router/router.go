// Arquivo: internal/api/router/router.go
package router

import (
	"github.com/gin-gonic/gin"
	"github.com/not-nullexception/image-optimizer/config"
	"github.com/not-nullexception/image-optimizer/internal/api/handlers"
	"github.com/not-nullexception/image-optimizer/internal/api/middleware" // Certifique-se que ambos os middlewares estão aqui
	"github.com/not-nullexception/image-optimizer/internal/db"
	"github.com/not-nullexception/image-optimizer/internal/minio"
	rabbitmq "github.com/not-nullexception/image-optimizer/internal/queue" // Use o nome correto do seu pacote
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func Setup(
	cfg *config.Config,
	repository db.Repository,
	minioClient minio.Client,
	queueClient rabbitmq.Client, // Use o nome correto do seu pacote
) *gin.Engine {
	// Set Gin mode
	gin.SetMode(cfg.Server.Mode)

	// Create router using New for custom middleware control
	r := gin.New()

	// --- Aplicar Middlewares Globais na Ordem Correta ---

	// 1. Tracing (se habilitado) - DEVE VIR PRIMEIRO
	if cfg.Tracing.Enabled {
		r.Use(otelgin.Middleware(cfg.Tracing.ServiceName))
	}

	// 2. Logger Contextual - DEVE VIR DEPOIS do Tracing
	//    Ele usará o trace_id/span_id se o tracing estiver habilitado.
	r.Use(middleware.ContextualLogger("api")) // Fornece um componente padrão

	// 3. Recuperação de Panics
	r.Use(gin.Recovery())

	// 4. CORS
	r.Use(middleware.CORS()) // Assumindo que você tem esse middleware

	// 5. Métricas (se habilitado)
	if cfg.Metrics.Enabled {
		r.Use(middleware.Metrics()) // Mantém o middleware de métricas separado
	}

	// 6. Opcional: Logger padrão do Gin (se ainda desejar)
	// r.Use(gin.Logger())

	// --- Criar Handlers (injeção de dependência) ---
	// Certifique-se que os handlers agora NÃO recebem/usam um logger diretamente
	imageHandler := handlers.NewImageHandler(repository, minioClient, queueClient, cfg)
	healthHandler := handlers.NewHealthHandler(repository)

	// --- Rotas ---
	// Health check
	r.GET("/health", healthHandler.Check) // Assumindo que o método é Check

	// Metrics endpoint (se habilitado)
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
		// Adicione outras rotas da API aqui dentro do grupo 'api'
	}

	return r
}
