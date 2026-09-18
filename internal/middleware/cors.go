package middleware

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS(allowedOrigins []string) gin.HandlerFunc{
	return cors.New(cors.Config{
		AllowOrigins: allowedOrigins,
		AllowMethods: []string{"GET","POST","PATCH","PUT","DELETE"},
		AllowHeaders: []string{"Authorization","Content-Type","Accept","Origin"},
	})
}