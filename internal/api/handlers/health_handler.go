package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/not-nullexception/image-optimizer/internal/db"
	"github.com/not-nullexception/image-optimizer/internal/logger"
	"github.com/rs/zerolog"
)

type HealthHandler struct {
	repo   db.Repository
	logger zerolog.Logger
}

type HeathResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	DB        string    `json:"db"`
}

func NewHealthHandler(repo db.Repository) *HealthHandler {
	return &HealthHandler{
		repo:   repo,
		logger: logger.GetLogger("health-handler"),
	}
}

// Check handles heath check requests
func (h *HealthHandler) Check(c *gin.Context) {
	response := HeathResponse{
		Status:    "UP",
		Timestamp: time.Now(),
		Version:   "1.0.0",
		DB:        "UP",
	}

	err := h.repo.Ping(c.Request.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("Database health check failed")
		response.Status = "DEGRADED"
		response.DB = "DOWN"
	}

	c.JSON(http.StatusOK, response)
}
