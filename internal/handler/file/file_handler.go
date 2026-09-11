package file

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/gin-gonic/gin"
	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
	"github.com/laraxpy/photo-bucket-backend/internal/service"
)

type FileHandler struct {
	fileService service.FileService
}

func NewFileHandler(fileService service.FileService) *FileHandler {
	return &FileHandler{fileService: fileService}
}

func (h *FileHandler) Upload(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.Error(apperror.BadRequest("no file provided", err))
		return
	}
	userIDValue, exists := c.Get("userID")
	if !exists {
		c.Error(apperror.Internal(nil))
		return
	}

	userIDStr, ok := userIDValue.(string)
	if !ok {
		c.Error(apperror.Internal(nil))
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.Error(apperror.Internal(err))
		return
	}

	openedFile, err := fileHeader.Open()
	if err != nil {
		c.Error(apperror.Internal(err))
		return
	}
	defer openedFile.Close()
	uploadedFile, err := h.fileService.Upload(
		c.Request.Context(),
		userID,
		openedFile,
		fileHeader.Filename,
		fileHeader.Header.Get("Content-Type"),
		fileHeader.Size,
	)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, uploadedFile)
}

func (h *FileHandler) List(c *gin.Context) {
	userIDValue, exists := c.Get("userID")
	if !exists {
		c.Error(apperror.Internal(nil))
		return
	}
	userIDStr, ok := userIDValue.(string)
	if !ok {
		c.Error(apperror.Internal(nil))
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.Error(apperror.Internal(err))
		return
	}

	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		c.Error(apperror.BadRequest("invalid limit", err))
		return
	}
	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		c.Error(apperror.BadRequest("invalid offset", err))
		return
	}

	files, err := h.fileService.ListByUser(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, files)
}
