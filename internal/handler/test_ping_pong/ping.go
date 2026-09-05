package test_ping_pong

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/laraxpy/go-backend-starter/internal/httpx"
)

type PingRequest struct {
	Name string `json:"name" binding:"required,min=8"`
}

func Ping(c *gin.Context){
	var req PingRequest
	validateRequest := httpx.BindAndValidate(c, &req)
	if !validateRequest {
	return
}	
	//simular lentitud con time.Sleep
	// time.Sleep(5 * time.Second)	
	c.JSON(http.StatusOK, gin.H{"message": "hola " + req.Name})
}