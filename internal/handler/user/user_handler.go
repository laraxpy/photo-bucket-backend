package user

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/laraxpy/photo-bucket-backend/internal/httpx"
	"github.com/laraxpy/photo-bucket-backend/internal/service"
)

type UserHandler struct {
	userService service.UserService
}

func NewUserHandler(userService service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

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

func (h *UserHandler) Login(c *gin.Context) {
	var req LoginRequest
	if !httpx.BindAndValidate(c, &req) {
		return
	}

	token, err := h.userService.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}
