package middleware

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc{
	return cors.New(cors.Config{
		AllowOrigins: []string{"http://localhost:4000"},
		AllowMethods: []string{"GET","POST","PATCH","PUT","DELETE"},
		AllowHeaders: []string{"Authorization","Content-Type","Accept","Origin"},
	})
}