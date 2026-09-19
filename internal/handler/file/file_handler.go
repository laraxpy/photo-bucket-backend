package file

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

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
//	@Description	Genera una URL firmada de MinIO valida por 15 minutos para la miniatura del archivo. Si la variante pedida no existe (tipo no soportado o fallo al generarla), cae a una variante mas chica y despues al original.
//	@Tags			files
//	@Produce		json
//	@Param			id		path		string	true	"ID del archivo"
//	@Param			size	query		string	false	"Variante: 'small' (listados) o 'medium' (photo viewer)"	Enums(small, medium)	default(small)
//	@Success		200		{object}	map[string]string	"url"
//	@Failure		400		{object}	apperror.ErrorResponse
//	@Failure		401		{object}	apperror.ErrorResponse
//	@Failure		404		{object}	apperror.ErrorResponse
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

	var size service.ThumbnailSize
	switch c.DefaultQuery("size", "small") {
	case "small":
		size = service.ThumbnailSizeSmall
	case "medium":
		size = service.ThumbnailSizeMedium
	default:
		c.Error(apperror.BadRequest("size must be 'small' or 'medium'", nil))
		return
	}

	thumbnailURL, err := h.fileService.ThumbnailURL(c.Request.Context(), userID, id, size)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": thumbnailURL})
}

// DownloadZip godoc
//
//	@Summary		Descarga masiva en zip
//	@Description	Arma un .zip en streaming con los archivos pedidos (por fileIds, por folderId, o ambos combinados) y lo devuelve como respuesta. Valida que todos los archivos existan y sean del usuario antes de escribir cualquier byte de la respuesta.
//	@Tags			files
//	@Accept			json
//	@Produce		application/zip
//	@Param			request	body	DownloadZipRequest	true	"Seleccion de archivos (al menos fileIds o folderId)"
//	@Success		200		{file}	binary	"application/zip"
//	@Failure		400		{object}	apperror.ErrorResponse
//	@Failure		401		{object}	apperror.ErrorResponse
//	@Failure		404		{object}	apperror.ErrorResponse
//	@Security		BearerAuth
//	@Router			/files/download-zip [post]
func (h *FileHandler) DownloadZip(c *gin.Context) {
	userID, err := httpx.UserIDFromContext(c)
	if err != nil {
		c.Error(err)
		return
	}

	var req DownloadZipRequest
	if !httpx.BindAndValidate(c, &req) {
		return
	}

	fileIDs := make([]uuid.UUID, 0, len(req.FileIDs))
	for _, idStr := range req.FileIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			c.Error(apperror.BadRequest("invalid file id in fileIds", err))
			return
		}
		fileIDs = append(fileIDs, id)
	}

	var folderID *uuid.UUID
	if req.FolderID != "" {
		parsed, err := uuid.Parse(req.FolderID)
		if err != nil {
			c.Error(apperror.BadRequest("invalid folderId", err))
			return
		}
		folderID = &parsed
	}

	if len(fileIDs) == 0 && folderID == nil {
		c.Error(apperror.BadRequest("must provide fileIds and/or folderId", nil))
		return
	}

	// Fail-fast: resolve and validate ownership of every file BEFORE
	// committing to a response. Once we write headers/status below, a
	// failure can no longer be reported as a clean JSON error.
	files, err := h.fileService.ResolveFilesForZip(c.Request.Context(), userID, fileIDs, folderID)
	if err != nil {
		c.Error(err)
		return
	}

	// A bulk zip can legitimately run past the server's default write
	// timeout depending on how many/how large the files are; extend it
	// instead of racing an arbitrary number of files against a fixed clock.
	if rc := http.NewResponseController(c.Writer); rc != nil {
		_ = rc.SetWriteDeadline(time.Time{})
	}

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", `attachment; filename="download.zip"`)
	c.Status(http.StatusOK)

	if err := h.fileService.StreamZip(c.Request.Context(), files, c.Writer); err != nil {
		slog.Error("fallo el streaming del zip de descarga masiva", "error", err)
	}
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
