package apperror

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

func FromValidationError(err error) *AppError {
	validationErrs, ok := errors.AsType[validator.ValidationErrors](err)
	if !ok {
		return Internal(err)
	}

	translatorMap := map[string]string{
		"required": "Este campo es obligatorio",
		"min":      "Debe tener como minimo %s caracteres",
		"max":      "Debe tener como maximo %s caracteres",
	}
	fields := make(map[string]string)

	for _, fieldErr := range validationErrs {
		message, exists := translatorMap[fieldErr.Tag()]
		if !exists {
			message = "Este campo no es valido"
		} else if strings.Contains(message, "%") {
			message = fmt.Sprintf(message, fieldErr.Param())
		}
		fields[fieldErr.Field()] = message
	}
	return Validation("Validation error", fields)
}
