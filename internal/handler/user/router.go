package user

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, userHandler *UserHandler) {
	r.POST("/user/register", userHandler.Register)
	r.POST("/user/login", userHandler.Login)
}
