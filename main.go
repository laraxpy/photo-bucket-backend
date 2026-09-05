package main

import (
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/laraxpy/go-backend-starter/internal/config"
	"github.com/laraxpy/go-backend-starter/internal/middleware"
	"github.com/laraxpy/go-backend-starter/internal/router"
	"github.com/laraxpy/go-backend-starter/internal/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	cfg := config.Load()
	r := gin.New()
	r.HandleMethodNotAllowed= true
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger())
	r.Use(middleware.CORS())
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.RateLimiter())
	router.RegisterRoutes(r)
	srv := server.New(r,cfg)
	server.Run(srv)
}