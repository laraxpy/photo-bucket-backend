package user

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/laraxpy/photo-bucket-backend/internal/httpx"
	usermodel "github.com/laraxpy/photo-bucket-backend/internal/model/user"
	"github.com/laraxpy/photo-bucket-backend/internal/service"
)

// El alias usermodel desambigua para swag: este paquete y internal/model/user
// se llaman ambos "user", y las anotaciones @Success de abajo referencian el modelo.
var _ usermodel.User

type UserHandler struct {
	userService service.UserService
}

func NewUserHandler(userService service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

// Register godoc
//
//	@Summary		Registrar un nuevo usuario
//	@Description	Crea una cuenta de usuario nueva con la contraseña hasheada
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			request	body		RegisterRequest	true	"Datos de registro"
//	@Success		201		{object}	usermodel.User
//	@Failure		400		{object}	apperror.ErrorResponse
//	@Failure		409		{object}	apperror.ErrorResponse
//	@Router			/user/register [post]
func (h *UserHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if !httpx.BindAndValidate(c, &req) {
		return
	}

	user, err := h.userService.Register(c.Request.Context(), req.Name, req.Email, req.Password)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, user)
}

// Login godoc
//
//	@Summary		Iniciar sesion
//	@Description	Valida las credenciales y devuelve un token JWT junto con un refresh token
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			request	body		LoginRequest	true	"Credenciales"
//	@Success		200		{object}	TokenResponse
//	@Failure		400		{object}	apperror.ErrorResponse
//	@Failure		401		{object}	apperror.ErrorResponse
//	@Router			/user/login [post]
func (h *UserHandler) Login(c *gin.Context) {
	var req LoginRequest
	if !httpx.BindAndValidate(c, &req) {
		return
	}

	tokens, err := h.userService.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, TokenResponse{Token: tokens.AccessToken, RefreshToken: tokens.RefreshToken})
}

// Refresh godoc
//
//	@Summary		Renovar sesion
//	@Description	Cambia un refresh token valido (y no usado) por un nuevo par de tokens
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			request	body		RefreshRequest	true	"Refresh token"
//	@Success		200		{object}	TokenResponse
//	@Failure		400		{object}	apperror.ErrorResponse
//	@Failure		401		{object}	apperror.ErrorResponse
//	@Router			/user/refresh [post]
func (h *UserHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if !httpx.BindAndValidate(c, &req) {
		return
	}

	tokens, err := h.userService.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, TokenResponse{Token: tokens.AccessToken, RefreshToken: tokens.RefreshToken})
}
