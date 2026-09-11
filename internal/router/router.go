package router

import (
	"github.com/gin-gonic/gin"
	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
	"github.com/laraxpy/photo-bucket-backend/internal/handler/file"
	"github.com/laraxpy/photo-bucket-backend/internal/handler/health"
	"github.com/laraxpy/photo-bucket-backend/internal/handler/test_ping_pong"
	"github.com/laraxpy/photo-bucket-backend/internal/handler/user"
)

func RegisterRoutes(r *gin.Engine, userHandler *user.UserHandler, fileHandler *file.FileHandler, jwtSecret string) {
	test_ping_pong.RegisterRoutes(r)
	health.RegisterRoutes(r)
	user.RegisterRoutes(r, userHandler)
	file.RegisterRoutes(r, fileHandler, jwtSecret)

	r.NoMethod(func(c *gin.Context) {
		c.Error(apperror.NoMethod("Method not allowed", nil))
	})
	r.NoRoute(func(c *gin.Context) {
		c.Error(apperror.NotFound("Page Not Found", nil))
	})
}
