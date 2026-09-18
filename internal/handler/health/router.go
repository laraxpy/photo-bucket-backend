package health

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, healthHandler *HealthHandler) {
	r.GET("/health", healthHandler.GetHealthStatus)
}
