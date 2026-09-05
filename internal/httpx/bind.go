package httpx

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
)

func BindAndValidate(c *gin.Context, req any) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		if _, ok := errors.AsType[validator.ValidationErrors](err); ok {
			c.Error(apperror.FromValidationError(err))
		} else {
			requiredFields := apperror.RequiredFieldsFrom(req)
			fields := make(map[string]string)
			for _, fieldName := range requiredFields {
				fields[fieldName] = "Este campo es requerido"
			}
			c.Error(apperror.Validation("faltan campos requeridos", fields))
		}
		return false
	}
	return true
}