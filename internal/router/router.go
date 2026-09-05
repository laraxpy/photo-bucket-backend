package router

import (
	"github.com/gin-gonic/gin"
	"github.com/laraxpy/go-backend-starter/internal/apperror"
	"github.com/laraxpy/go-backend-starter/internal/handler/health"
	"github.com/laraxpy/go-backend-starter/internal/handler/test_ping_pong"
)

func RegisterRoutes(r *gin.Engine) {
	test_ping_pong.RegisterRoutes(r)
	health.RegisterRoutes(r)
	


	r.NoMethod(func(c *gin.Context){
		c.Error(apperror.NoMethod("Metodo no permitido", nil))
	})
	r.NoRoute(func(c *gin.Context){
		c.Error(apperror.NotFound("Page Not Found", nil))
	})
}