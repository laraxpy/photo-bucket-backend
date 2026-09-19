package config

import (
	"log/slog"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

var defaultAllowedOrigins = []string{"http://localhost:4000"}

type Config struct {
	Port           string
	DatabaseURL    string
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	JWTSecret      string
	AllowedOrigins []string
	RateLimit      string
}

func Load() *Config {
	err := godotenv.Load()
	if err != nil {
		slog.Warn("Archivo .env no econctrado, usando variables del sistema", "error", err)
	}
	return &Config{
		Port:           os.Getenv("PORT"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		MinioEndpoint:  os.Getenv("MINIO_ENDPOINT"),
		MinioAccessKey: os.Getenv("MINIO_ACCESS_KEY"),
		MinioSecretKey: os.Getenv("MINIO_SECRET_KEY"),
		MinioBucket:    os.Getenv("MINIO_BUCKET"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		AllowedOrigins: parseOrigins(os.Getenv("CORS_ALLOWED_ORIGINS")),
		RateLimit:      os.Getenv("RATE_LIMIT"),
	}
}

func parseOrigins(raw string) []string {
	if raw == "" {
		return defaultAllowedOrigins
	}
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	if len(origins) == 0 {
		return defaultAllowedOrigins
	}
	return origins
}
