package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
)

func AuthRequired(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.Error(apperror.Unauthorized("Authorization no present", nil))
			c.Abort()
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")

		parsedToken, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
			return []byte(jwtSecret), nil
		})
		if err != nil || !parsedToken.Valid {
			c.Error(apperror.Unauthorized("Invalid token", nil))
			c.Abort()
			return
		}
		claims, ok := parsedToken.Claims.(jwt.MapClaims)
		if !ok {
			c.Error(apperror.Internal(nil))
			c.Abort()
			return
		}

		userID, ok := claims["sub"].(string)
		if !ok {
			c.Error(apperror.Internal(nil))
			c.Abort()
			return
		}
		c.Set("userID", userID)
		c.Next()
	}

}
