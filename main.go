package main

import (
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/laraxpy/photo-bucket-backend/internal/config"
	"github.com/laraxpy/photo-bucket-backend/internal/database"
	"github.com/laraxpy/photo-bucket-backend/internal/handler/user"
	"github.com/laraxpy/photo-bucket-backend/internal/middleware"
	"github.com/laraxpy/photo-bucket-backend/internal/router"
	"github.com/laraxpy/photo-bucket-backend/internal/server"
	service "github.com/laraxpy/photo-bucket-backend/internal/service/user_service"
	"github.com/laraxpy/photo-bucket-backend/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	cfg := config.Load()
	db := database.Connect(cfg)
	userStore := store.NewUserStore(db)
	userService := service.NewUserService(userStore, cfg.JWTSecret)
	userHandler := user.NewUserHandler(userService)
	r := gin.New()
	r.HandleMethodNotAllowed = true
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger())
	r.Use(middleware.CORS())
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.RateLimiter())
	router.RegisterRoutes(r, userHandler)
	srv := server.New(r, cfg)
	server.Run(srv)
}
