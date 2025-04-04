package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/not-nullexception/image-optimizer/internal/metrics"
)

// Metrics returns a middleware for collection metrics
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		// Process request
		c.Next()

		// Calculate request duration
		duration := time.Since(start).Seconds()

		// Record metrics
		method := c.Request.Method
		status := strconv.Itoa(c.Writer.Status())

		// Track total requests
		metrics.RequestsTotal.WithLabelValues(method, status, path).Inc()

		// Track request duration
		metrics.RequestDuration.WithLabelValues(method, path).Observe(duration)
	}
}
