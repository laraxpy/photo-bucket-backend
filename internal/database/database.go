package database

import (
	"log/slog"
	"os"

	"github.com/laraxpy/photo-bucket-backend/internal/config"
	"github.com/laraxpy/photo-bucket-backend/internal/model/file"
	"github.com/laraxpy/photo-bucket-backend/internal/model/user"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	// "gorm.io/gorm/logger"
)

func Connect(cfg *config.Config) *gorm.DB {
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		// Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		slog.Error("cannot connect to the database ", "error", err)
		os.Exit(1)
	}
	err = db.AutoMigrate(user.User{}, file.File{})
	if err != nil {
		slog.Error("Automigration failed", "error", err)
		os.Exit(1)

	}
	return db
}
