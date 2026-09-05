package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

func RequestLogger() gin.HandlerFunc{
	return func(c *gin.Context){
		start := time.Now()

		c.Next()

		duration := time.Since(start)

		slog.Info("request completed",
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"status", c.Writer.Status(),
		"duration_ms", duration.Milliseconds(),
		"client_ip", c.ClientIP(),
		)
	}
}