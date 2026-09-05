package middleware

import (
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/laraxpy/go-backend-starter/internal/apperror"
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

func RateLimiter() gin.HandlerFunc{
	rate, err := limiter.NewRateFromFormatted("10-S")
	if err != nil {
		slog.Error("Rate limit no establecido", "error", err)
		os.Exit(1)
	}
	store := memory.NewStore()
	instance := limiter.New(store, rate)
	return func(c *gin.Context){
		limiterCtx, err := instance.Get(c.Request.Context(),c.ClientIP())
		if err != nil{
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