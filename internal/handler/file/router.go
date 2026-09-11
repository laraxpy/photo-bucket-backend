package file

import (
	"github.com/gin-gonic/gin"
	"github.com/laraxpy/photo-bucket-backend/internal/middleware"
)

func RegisterRoutes(r *gin.Engine, fileHandler *FileHandler, jwtSecret string) {
	r.POST("/files/upload", middleware.AuthRequired(jwtSecret), fileHandler.Upload)
	r.GET("/files/list", middleware.AuthRequired(jwtSecret), fileHandler.List)
}
