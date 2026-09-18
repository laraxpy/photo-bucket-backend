package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
	"gorm.io/gorm"
)

type HealthHandler struct {
	db         *gorm.DB
	minio      *minio.Client
	bucketName string
}

func NewHealthHandler(db *gorm.DB, minioClient *minio.Client, bucketName string) *HealthHandler {
	return &HealthHandler{db: db, minio: minioClient, bucketName: bucketName}
}

// GetHealthStatus godoc
//
//	@Summary		Estado del servidor
//	@Description	Verifica conectividad real con Postgres y MinIO, no solo que el proceso este vivo
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	map[string]string
//	@Failure		503	{object}	map[string]string
//	@Router			/health [get]
func (h *HealthHandler) GetHealthStatus(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	status := gin.H{"server": "ok"}
	healthy := true

	if sqlDB, err := h.db.DB(); err != nil || sqlDB.PingContext(ctx) != nil {
		status["database"] = "unreachable"
		healthy = false
	} else {
		status["database"] = "ok"
	}

	if _, err := h.minio.BucketExists(ctx, h.bucketName); err != nil {
		status["storage"] = "unreachable"
		healthy = false
	} else {
		status["storage"] = "ok"
	}

	if !healthy {
		c.JSON(http.StatusServiceUnavailable, status)
		return
	}
	c.JSON(http.StatusOK, status)
}
