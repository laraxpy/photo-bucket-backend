package folder

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
	"github.com/laraxpy/photo-bucket-backend/internal/httpx"
	"github.com/laraxpy/photo-bucket-backend/internal/service"
)

type FolderHandler struct {
	folderService service.FolderService
}

func NewFolderHandler(folderService service.FolderService) *FolderHandler {
	return &FolderHandler{folderService: folderService}
}

func (h *FolderHandler) Create(c *gin.Context) {
	userID, err := httpx.UserIDFromContext(c)
	if err != nil {
		c.Error(err)
		return
	}

	var req CreateFolderRequest
	if !httpx.BindAndValidate(c, &req) {
		return
	}

	f, err := h.folderService.Create(c.Request.Context(), userID, req.Name, req.ParentID)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, f)
}

func (h *FolderHandler) List(c *gin.Context) {
	userID, err := httpx.UserIDFromContext(c)
	if err != nil {
		c.Error(err)
		return
	}

	folders, err := h.folderService.ListByUser(c.Request.Context(), userID, c.Query("parentId"))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, folders)
}

func (h *FolderHandler) GetByID(c *gin.Context) {
	userID, err := httpx.UserIDFromContext(c)
	if err != nil {
		c.Error(err)
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Error(apperror.BadRequest("invalid folder id", err))
		return
	}

	f, err := h.folderService.GetByID(c.Request.Context(), userID, id)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, f)
}

func (h *FolderHandler) Rename(c *gin.Context) {
	userID, err := httpx.UserIDFromContext(c)
	if err != nil {
		c.Error(err)
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Error(apperror.BadRequest("invalid folder id", err))
		return
	}

	var req RenameFolderRequest
	if !httpx.BindAndValidate(c, &req) {
		return
	}

	f, err := h.folderService.Rename(c.Request.Context(), userID, id, req.Name)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, f)
}

func (h *FolderHandler) Move(c *gin.Context) {
	userID, err := httpx.UserIDFromContext(c)
	if err != nil {
		c.Error(err)
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Error(apperror.BadRequest("invalid folder id", err))
		return
	}

	var req MoveFolderRequest
	if !httpx.BindAndValidate(c, &req) {
		return
	}

	f, err := h.folderService.Move(c.Request.Context(), userID, id, req.ParentID)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, f)
}

func (h *FolderHandler) Delete(c *gin.Context) {
	userID, err := httpx.UserIDFromContext(c)
	if err != nil {
		c.Error(err)
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Error(apperror.BadRequest("invalid folder id", err))
		return
	}

	if err := h.folderService.Delete(c.Request.Context(), userID, id); err != nil {
		c.Error(err)
		return
	}
	c.Status(http.StatusNoContent)
}
