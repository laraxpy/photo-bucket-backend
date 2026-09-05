package middleware

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/laraxpy/go-backend-starter/internal/apperror"
)

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}

		err := c.Errors.Last().Err
		appErr := apperror.From(err)

		if appErr.HTTPStatus >= http.StatusInternalServerError {
			slog.Error("internal error",
		"code", appErr.Code,
		"path", c.Request.URL.Path,
		"error", appErr.Err,
	)
		}

		response := gin.H{
			"error": gin.H{
				"code":    appErr.Code,
				"message": appErr.Message,
			},
		}
		if len(appErr.Fields) > 0 {
			response["error"].(gin.H)["fields"] = appErr.Fields
		}

		c.AbortWithStatusJSON(appErr.HTTPStatus, response)
	}
}