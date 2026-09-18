package router

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
	"github.com/laraxpy/photo-bucket-backend/internal/handler/file"
	"github.com/laraxpy/photo-bucket-backend/internal/handler/folder"
	"github.com/laraxpy/photo-bucket-backend/internal/handler/health"
	"github.com/laraxpy/photo-bucket-backend/internal/handler/test_ping_pong"
	"github.com/laraxpy/photo-bucket-backend/internal/handler/user"
)

func RegisterRoutes(r *gin.Engine, userHandler *user.UserHandler, fileHandler *file.FileHandler, folderHandler *folder.FolderHandler, jwtSecret string) {
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	test_ping_pong.RegisterRoutes(r)
	health.RegisterRoutes(r)
	user.RegisterRoutes(r, userHandler)
	file.RegisterRoutes(r, fileHandler, jwtSecret)
	folder.RegisterRoutes(r, folderHandler, jwtSecret)

	r.NoMethod(func(c *gin.Context) {
		c.Error(apperror.NoMethod("Method not allowed", nil))
	})
	r.NoRoute(func(c *gin.Context) {
		c.Error(apperror.NotFound("Page Not Found", nil))
	})
}
