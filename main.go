package main

import (
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/laraxpy/photo-bucket-backend/internal/config"
	"github.com/laraxpy/photo-bucket-backend/internal/database"
	"github.com/laraxpy/photo-bucket-backend/internal/handler/file"
	"github.com/laraxpy/photo-bucket-backend/internal/handler/folder"
	"github.com/laraxpy/photo-bucket-backend/internal/handler/user"
	"github.com/laraxpy/photo-bucket-backend/internal/middleware"
	"github.com/laraxpy/photo-bucket-backend/internal/router"
	"github.com/laraxpy/photo-bucket-backend/internal/server"
	"github.com/laraxpy/photo-bucket-backend/internal/service"
	"github.com/laraxpy/photo-bucket-backend/internal/storage"
	"github.com/laraxpy/photo-bucket-backend/internal/store"

	_ "github.com/laraxpy/photo-bucket-backend/docs"
)

// @title						Photo Bucket Backend API
// @version					1.0
// @description				API para gestionar lectura y escritura de archivos multimedia (fotos) usando MinIO como almacenamiento de objetos.
// @BasePath					/
// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
// @description				Escribir "Bearer" seguido de un espacio y el token JWT obtenido en /user/login.
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	cfg := config.Load()
	db := database.Connect(cfg)
	userStore := store.NewUserStore(db)
	userService := service.NewUserService(userStore, cfg.JWTSecret)
	userHandler := user.NewUserHandler(userService)
	minioClient := storage.Connect(cfg)
	fileStore := store.NewFileStore(db)
	folderStore := store.NewFolderStore(db)
	folderService := service.NewFolderService(folderStore, fileStore)
	folderHandler := folder.NewFolderHandler(folderService)
	fileService := service.NewFileService(fileStore, folderStore, minioClient, cfg.MinioBucket)
	fileHandler := file.NewFileHandler(fileService)
	r := gin.New()
	r.HandleMethodNotAllowed = true
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger())
	r.Use(middleware.CORS())
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.RateLimiter())
	router.RegisterRoutes(r, userHandler, fileHandler, folderHandler, cfg.JWTSecret)
	srv := server.New(r, cfg)
	server.Run(srv)
}
