package folder

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
	"github.com/laraxpy/photo-bucket-backend/internal/httpx"
	foldermodel "github.com/laraxpy/photo-bucket-backend/internal/model/folder"
	"github.com/laraxpy/photo-bucket-backend/internal/service"
)

// El alias foldermodel desambigua para swag: este paquete y internal/model/folder
// se llaman ambos "folder", y las anotaciones @Success de abajo referencian el modelo.
var _ foldermodel.Folder

type FolderHandler struct {
	folderService service.FolderService
}

func NewFolderHandler(folderService service.FolderService) *FolderHandler {
	return &FolderHandler{folderService: folderService}
}

// Create godoc
//
//	@Summary		Crear una carpeta
//	@Description	Crea una carpeta para el usuario autenticado, opcionalmente dentro de otra carpeta (parentId)
//	@Tags			folders
//	@Accept			json
//	@Produce		json
//	@Param			request	body		CreateFolderRequest	true	"Datos de la carpeta"
//	@Success		201		{object}	foldermodel.Folder
//	@Failure		400		{object}	apperror.ErrorResponse
//	@Failure		401		{object}	apperror.ErrorResponse
//	@Failure		404		{object}	apperror.ErrorResponse	"parentId no existe o no pertenece al usuario"
//	@Failure		409		{object}	apperror.ErrorResponse	"ya existe una carpeta con ese nombre en el mismo padre"
//	@Security		BearerAuth
//	@Router			/folders [post]
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

// List godoc
//
//	@Summary		Listar carpetas
//	@Description	Lista las carpetas del usuario autenticado. Sin parentId, lista las carpetas raiz
//	@Tags			folders
//	@Produce		json
//	@Param			parentId	query		string	false	"Listar el contenido de esta carpeta"
//	@Success		200			{array}		foldermodel.Folder
//	@Failure		400			{object}	apperror.ErrorResponse
//	@Failure		401			{object}	apperror.ErrorResponse
//	@Failure		404			{object}	apperror.ErrorResponse	"parentId no existe o no pertenece al usuario"
//	@Security		BearerAuth
//	@Router			/folders [get]
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

// GetByID godoc
//
//	@Summary		Obtener una carpeta
//	@Tags			folders
//	@Produce		json
//	@Param			id	path		string	true	"ID de la carpeta"
//	@Success		200	{object}	foldermodel.Folder
//	@Failure		400	{object}	apperror.ErrorResponse
//	@Failure		401	{object}	apperror.ErrorResponse
//	@Failure		404	{object}	apperror.ErrorResponse
//	@Security		BearerAuth
//	@Router			/folders/{id} [get]
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

// Rename godoc
//
//	@Summary		Renombrar una carpeta
//	@Tags			folders
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"ID de la carpeta"
//	@Param			request	body		RenameFolderRequest	true	"Nuevo nombre"
//	@Success		200		{object}	foldermodel.Folder
//	@Failure		400		{object}	apperror.ErrorResponse
//	@Failure		401		{object}	apperror.ErrorResponse
//	@Failure		404		{object}	apperror.ErrorResponse
//	@Failure		409		{object}	apperror.ErrorResponse	"ya existe una carpeta con ese nombre en el mismo padre"
//	@Security		BearerAuth
//	@Router			/folders/{id} [patch]
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

// Move godoc
//
//	@Summary		Mover una carpeta
//	@Description	Cambia el padre de la carpeta. parentId vacio la mueve a la raiz
//	@Tags			folders
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"ID de la carpeta"
//	@Param			request	body		MoveFolderRequest	true	"Nuevo padre"
//	@Success		200		{object}	foldermodel.Folder
//	@Failure		400		{object}	apperror.ErrorResponse	"parentId invalido o crearia un ciclo"
//	@Failure		401		{object}	apperror.ErrorResponse
//	@Failure		404		{object}	apperror.ErrorResponse
//	@Failure		409		{object}	apperror.ErrorResponse	"ya existe una carpeta con ese nombre en el destino"
//	@Security		BearerAuth
//	@Router			/folders/{id}/move [patch]
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

// Delete godoc
//
//	@Summary		Eliminar una carpeta
//	@Description	Elimina la carpeta si esta vacia. Si tiene subcarpetas o archivos, devuelve 409
//	@Tags			folders
//	@Param			id	path	string	true	"ID de la carpeta"
//	@Success		204	"sin contenido"
//	@Failure		400	{object}	apperror.ErrorResponse
//	@Failure		401	{object}	apperror.ErrorResponse
//	@Failure		404	{object}	apperror.ErrorResponse
//	@Failure		409	{object}	apperror.ErrorResponse	"la carpeta tiene subcarpetas o archivos"
//	@Security		BearerAuth
//	@Router			/folders/{id} [delete]
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
