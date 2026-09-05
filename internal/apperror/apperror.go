package apperror

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
)

type ErrorCode string

const (
	CodeBadRequest   ErrorCode = "BAD_REQUEST"
	CodeUnauthorized ErrorCode = "UNAUTHORIZED"
	CodeForbidden    ErrorCode = "FORBIDDEN"
	CodeNotFound     ErrorCode = "NOT_FOUND"
	CodeConflict     ErrorCode = "CONFLICT"
	CodeValidation   ErrorCode = "VALIDATION_ERROR"
	CodeInternal     ErrorCode = "INTERNAL_ERROR"
	CodeNoMethod	 ErrorCode = "NO_METHOD"
	CodeTooManyRequest ErrorCode = "TOO_MANY_REQUEST"
)

type AppError struct {
	Code       ErrorCode
	Message    string
	HTTPStatus int
	Err        error
	Fields     map[string]string
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error { return e.Err }

func BadRequest(message string, err error) *AppError {
	return &AppError{Code: CodeBadRequest, Message: message, HTTPStatus: http.StatusBadRequest, Err: err}
}

func Unauthorized(message string, err error) *AppError {
	return &AppError{Code: CodeUnauthorized, Message: message, HTTPStatus: http.StatusUnauthorized, Err: err}
}

func Forbidden(message string, err error) *AppError {
	return &AppError{Code: CodeForbidden, Message: message, HTTPStatus: http.StatusForbidden, Err: err}
}

func NotFound(message string, err error) *AppError {
	return &AppError{Code: CodeNotFound, Message: message, HTTPStatus: http.StatusNotFound, Err: err}
}

func Conflict(message string, err error) *AppError {
	return &AppError{Code: CodeConflict, Message: message, HTTPStatus: http.StatusConflict, Err: err}
}

func Validation(message string, fields map[string]string) *AppError {
	return &AppError{Code: CodeValidation, Message: message, HTTPStatus: http.StatusBadRequest, Fields: fields}
}

func NoMethod(message string, err error) *AppError{
	return &AppError{Code: CodeNoMethod, Message: message, HTTPStatus: http.StatusMethodNotAllowed, Err: err}
}
func Internal(err error) *AppError {
	return &AppError{Code: CodeInternal, Message: "internal server error", HTTPStatus: http.StatusInternalServerError, Err: err}
}
func TooManyRequest(message string, err error) *AppError{
	return &AppError{Code: CodeTooManyRequest, Message: message, HTTPStatus: http.StatusTooManyRequests, Err: err }
}

func From(err error) *AppError {
	if err == nil {
		return nil
	}

	if appErr, ok := errors.AsType[*AppError](err); ok {
		return appErr
	}

	return Internal(err)
}

func RequiredFieldsFrom(v any) []string {
	t := reflect.TypeOf(v)
	var required []string
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		bindingTag := field.Tag.Get("binding")
		if strings.Contains(bindingTag, "required") {
			required = append(required, field.Name)
		}
	}
	return required
}