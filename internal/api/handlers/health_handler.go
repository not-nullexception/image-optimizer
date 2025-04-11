package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/not-nullexception/image-optimizer/internal/db"
	"github.com/not-nullexception/image-optimizer/internal/logger"
)

type HealthHandler struct {
	repo db.Repository
}

type HeathResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	DB        string    `json:"db"`
}

func NewHealthHandler(repo db.Repository) *HealthHandler {
	return &HealthHandler{
		repo: repo,
	}
}

// Check handles heath check requests
func (h *HealthHandler) Check(c *gin.Context) {
	reqLogger := logger.FromContext(c.Request.Context())
	reqLogger.Info().Msg("Processing health check request")

	response := HeathResponse{
		Status:    "UP",
		Timestamp: time.Now(),
		Version:   "1.0.0",
		DB:        "UP",
	}

	err := h.repo.Ping(c.Request.Context())
	if err != nil {
		reqLogger.Error().Err(err).Msg("Database health check failed")
		response.Status = "DEGRADED"
		response.DB = "DOWN"
	}

	reqLogger.Info().Msg("Health check successful")
	c.JSON(http.StatusOK, response)
}
