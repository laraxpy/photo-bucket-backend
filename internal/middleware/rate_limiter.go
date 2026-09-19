package middleware

import (
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

const defaultRateFormatted = "30-S"

// RateLimiter limita requests por IP. rateFormatted usa el formato de
// ulule/limiter (ej. "30-S" = 30 por segundo); si viene vacío se usa
// defaultRateFormatted.
func RateLimiter(rateFormatted string) gin.HandlerFunc {
	if rateFormatted == "" {
		rateFormatted = defaultRateFormatted
	}
	rate, err := limiter.NewRateFromFormatted(rateFormatted)
	if err != nil {
		slog.Error("Rate limit no establecido", "error", err)
		os.Exit(1)
	}
	store := memory.NewStore()
	instance := limiter.New(store, rate)
	return func(c *gin.Context) {
		limiterCtx, err := instance.Get(c.Request.Context(), c.ClientIP())
		if err != nil {
			c.Error(apperror.Internal(err))
			c.Abort()
			return
		}
		if limiterCtx.Reached {
			c.Error(apperror.TooManyRequest("Too many request", nil))
			c.Abort()
			return
		}
		c.Next()
	}
}
