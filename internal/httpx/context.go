package httpx

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
)

func UserIDFromContext(c *gin.Context) (uuid.UUID, error) {
	userIDValue, exists := c.Get("userID")
	if !exists {
		return uuid.Nil, apperror.Internal(nil)
	}

	userIDStr, ok := userIDValue.(string)
	if !ok {
		return uuid.Nil, apperror.Internal(nil)
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, apperror.Internal(err)
	}
	return userID, nil
}
