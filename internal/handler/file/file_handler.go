package file

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/gin-gonic/gin"
	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
	"github.com/laraxpy/photo-bucket-backend/internal/httpx"
	filemodel "github.com/laraxpy/photo-bucket-backend/internal/model/file"
	"github.com/laraxpy/photo-bucket-backend/internal/service"
)

const maxListLimit = 100

// El alias filemodel desambigua para swag: este paquete y internal/model/file
// se llaman ambos "file", y las anotaciones @Success de abajo referencian el modelo.
var _ filemodel.File

type FileHandler struct {
	fileService service.FileService
}

func NewFileHandler(fileService service.FileService) *FileHandler {
	return &FileHandler{fileService: fileService}
}

// Upload godoc
//
//	@Summary		Subir un archivo
//	@Description	Sube un archivo multimedia a MinIO y guarda sus metadatos. Opcionalmente puede asociarse a una carpeta del usuario.
//	@Tags			files
//	@Accept			mpfd
//	@Produce		json
//	@Param			file		formData	file	true	"Archivo a subir"
//	@Param			folderId	formData	string	false	"ID de la carpeta destino (vacio = raiz)"
//	@Success		201			{object}	filemodel.File
//	@Failure		400			{object}	apperror.ErrorResponse
//	@Failure		401			{object}	apperror.ErrorResponse
//	@Failure		404			{object}	apperror.ErrorResponse	"la carpeta indicada no existe o no pertenece al usuario"
//	@Security		BearerAuth
//	@Router			/files/upload [post]
func (h *FileHandler) Upload(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.Error(apperror.BadRequest("no file provided", err))
		return
	}

	userID, err := httpx.UserIDFromContext(c)
	if err != nil {
		c.Error(err)
		return
	}

	var folderID *uuid.UUID
	if folderIDStr := c.PostForm("folderId"); folderIDStr != "" {
		parsed, err := uuid.Parse(folderIDStr)
		if err != nil {
			c.Error(apperror.BadRequest("invalid folderId", err))
			return
		}
		folderID = &parsed
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
		folderID,
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

// List godoc
//
//	@Summary		Listar archivos del usuario
//	@Description	Lista los archivos del usuario autenticado, opcionalmente filtrados por carpeta
//	@Tags			files
//	@Produce		json
//	@Param			folderId	query		string	false	"Filtrar por carpeta"
//	@Param			limit		query		int		false	"Cantidad maxima de resultados (1-100)"	default(20)
//	@Param			offset		query		int		false	"Desplazamiento para paginacion"	default(0)
//	@Success		200			{array}		filemodel.File
//	@Failure		400			{object}	apperror.ErrorResponse
//	@Failure		401			{object}	apperror.ErrorResponse
//	@Security		BearerAuth
//	@Router			/files/list [get]
func (h *FileHandler) List(c *gin.Context) {
	userID, err := httpx.UserIDFromContext(c)
	if err != nil {
		c.Error(err)
		return
	}

	var folderID *uuid.UUID
	if folderIDStr := c.Query("folderId"); folderIDStr != "" {
		parsed, err := uuid.Parse(folderIDStr)
		if err != nil {
			c.Error(apperror.BadRequest("invalid folderId", err))
			return
		}
		folderID = &parsed
	}

	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > maxListLimit {
		c.Error(apperror.BadRequest(fmt.Sprintf("limit must be between 1 and %d", maxListLimit), err))
		return
	}
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		c.Error(apperror.BadRequest("offset must be a non-negative integer", err))
		return
	}

	files, err := h.fileService.ListByUser(c.Request.Context(), userID, folderID, limit, offset)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, files)
}

// GetByID godoc
//
//	@Summary		Obtener metadata de un archivo
//	@Tags			files
//	@Produce		json
//	@Param			id	path		string	true	"ID del archivo"
//	@Success		200	{object}	filemodel.File
//	@Failure		400	{object}	apperror.ErrorResponse
//	@Failure		401	{object}	apperror.ErrorResponse
//	@Failure		404	{object}	apperror.ErrorResponse
//	@Security		BearerAuth
//	@Router			/files/{id} [get]
func (h *FileHandler) GetByID(c *gin.Context) {
	userID, err := httpx.UserIDFromContext(c)
	if err != nil {
		c.Error(err)
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Error(apperror.BadRequest("invalid file id", err))
		return
	}

	f, err := h.fileService.GetByID(c.Request.Context(), userID, id)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, f)
}

// DownloadURL godoc
//
//	@Summary		Obtener URL de descarga
//	@Description	Genera una URL firmada de MinIO valida por 15 minutos para ver o descargar el archivo
//	@Tags			files
//	@Produce		json
//	@Param			id	path		string	true	"ID del archivo"
//	@Success		200	{object}	map[string]string	"url"
//	@Failure		400	{object}	apperror.ErrorResponse
//	@Failure		401	{object}	apperror.ErrorResponse
//	@Failure		404	{object}	apperror.ErrorResponse
//	@Security		BearerAuth
//	@Router			/files/{id}/url [get]
func (h *FileHandler) DownloadURL(c *gin.Context) {
	userID, err := httpx.UserIDFromContext(c)
	if err != nil {
		c.Error(err)
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Error(apperror.BadRequest("invalid file id", err))
		return
	}

	downloadURL, err := h.fileService.DownloadURL(c.Request.Context(), userID, id)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": downloadURL})
}

// ThumbnailURL godoc
//
//	@Summary		Obtener URL de la miniatura
//	@Description	Genera una URL firmada de MinIO valida por 15 minutos para la miniatura del archivo. Si el archivo no tiene miniatura (tipo no soportado o fallo al generarla), devuelve la URL del original como fallback.
//	@Tags			files
//	@Produce		json
//	@Param			id	path		string	true	"ID del archivo"
//	@Success		200	{object}	map[string]string	"url"
//	@Failure		400	{object}	apperror.ErrorResponse
//	@Failure		401	{object}	apperror.ErrorResponse
//	@Failure		404	{object}	apperror.ErrorResponse
//	@Security		BearerAuth
//	@Router			/files/{id}/thumbnail-url [get]
func (h *FileHandler) ThumbnailURL(c *gin.Context) {
	userID, err := httpx.UserIDFromContext(c)
	if err != nil {
		c.Error(err)
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Error(apperror.BadRequest("invalid file id", err))
		return
	}

	thumbnailURL, err := h.fileService.ThumbnailURL(c.Request.Context(), userID, id)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": thumbnailURL})
}

// Delete godoc
//
//	@Summary		Eliminar un archivo
//	@Description	Elimina el objeto en MinIO y marca el registro como borrado
//	@Tags			files
//	@Param			id	path	string	true	"ID del archivo"
//	@Success		204	"sin contenido"
//	@Failure		400	{object}	apperror.ErrorResponse
//	@Failure		401	{object}	apperror.ErrorResponse
//	@Failure		404	{object}	apperror.ErrorResponse
//	@Security		BearerAuth
//	@Router			/files/{id} [delete]
func (h *FileHandler) Delete(c *gin.Context) {
	userID, err := httpx.UserIDFromContext(c)
	if err != nil {
		c.Error(err)
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Error(apperror.BadRequest("invalid file id", err))
		return
	}

	if err := h.fileService.Delete(c.Request.Context(), userID, id); err != nil {
		c.Error(err)
		return
	}
	c.Status(http.StatusNoContent)
}
