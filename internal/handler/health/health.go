package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetHealthStatus godoc
//
//	@Summary	Estado del servidor
//	@Tags		health
//	@Produce	json
//	@Success	200	{object}	map[string]string
//	@Router		/health [get]
func GetHealthStatus(c *gin.Context){
	c.JSON(http.StatusOK, gin.H{"serverStatus":"Ok"})
}