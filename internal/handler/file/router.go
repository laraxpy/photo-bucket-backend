package file

import (
	"github.com/gin-gonic/gin"
	"github.com/laraxpy/photo-bucket-backend/internal/middleware"
)

func RegisterRoutes(r *gin.Engine, fileHandler *FileHandler, jwtSecret string) {
	r.POST("/files/upload", middleware.AuthRequired(jwtSecret), fileHandler.Upload)
	r.POST("/files/download-zip", middleware.AuthRequired(jwtSecret), fileHandler.DownloadZip)
	r.GET("/files/list", middleware.AuthRequired(jwtSecret), fileHandler.List)
	r.GET("/files/:id", middleware.AuthRequired(jwtSecret), fileHandler.GetByID)
	r.GET("/files/:id/url", middleware.AuthRequired(jwtSecret), fileHandler.DownloadURL)
	r.GET("/files/:id/thumbnail-url", middleware.AuthRequired(jwtSecret), fileHandler.ThumbnailURL)
	r.DELETE("/files/:id", middleware.AuthRequired(jwtSecret), fileHandler.Delete)
}
