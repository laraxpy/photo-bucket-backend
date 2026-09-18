package folder

import (
	"github.com/gin-gonic/gin"
	"github.com/laraxpy/photo-bucket-backend/internal/middleware"
)

func RegisterRoutes(r *gin.Engine, folderHandler *FolderHandler, jwtSecret string) {
	group := r.Group("/folders", middleware.AuthRequired(jwtSecret))
	group.POST("", folderHandler.Create)
	group.GET("", folderHandler.List)
	group.GET("/:id", folderHandler.GetByID)
	group.PATCH("/:id", folderHandler.Rename)
	group.PATCH("/:id/move", folderHandler.Move)
	group.DELETE("/:id", folderHandler.Delete)
}
