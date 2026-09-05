package config

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	DatabaseURL   string
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
}

func Load() *Config {
	err := godotenv.Load()
	if err != nil{
		slog.Warn("Archivo .env no econctrado, usando variables del sistema", "error", err)
	}
	return &Config{
	Port: os.Getenv("PORT"),
	DatabaseURL: os.Getenv("DATABASE_URL"),
	MinioEndpoint: os.Getenv("MINIO_ENDPOINT"),
	MinioAccessKey: os.Getenv("MINIO_ACCESS_KEY"),
	MinioSecretKey: os.Getenv("MINIO_SECRET_KEY"),
	MinioBucket: os.Getenv("MINIO_BUCKET"),
	}
}