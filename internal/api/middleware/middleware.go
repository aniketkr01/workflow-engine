package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/aniketkr01/workflow-engine/internal/logger"
	"github.com/aniketkr01/workflow-engine/internal/telemetry"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RequestLogger logs each incoming HTTP request with structured fields.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		dur := time.Since(start)

		corrID, _ := c.Get("correlation_id")
		logger.Info(c, "request logged",
			zap.Any("method", c.Request.Method),
			zap.String("path", c.FullPath()),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", dur),
			zap.String("client_ip", c.ClientIP()),
			zap.String("correlation_id", fmt.Sprintf("%v", corrID)),
			zap.String("msg", "request"))
	}
}

// PrometheusMiddleware records HTTP metrics.
func PrometheusMiddleware(metrics *telemetry.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		dur := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}
		metrics.HTTPRequestDuration.WithLabelValues(c.Request.Method, path, status).Observe(dur)
		metrics.HTTPRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
	}
}

// Recovery handles panics and returns 500.
func Recovery() gin.HandlerFunc {
	return gin.RecoveryWithWriter(nil, func(c *gin.Context, err any) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal server error",
		})
	})
}
