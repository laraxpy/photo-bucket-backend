package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetHealthStatus(c *gin.Context){
	c.JSON(http.StatusOK, gin.H{"serverStatus":"Ok"})
}