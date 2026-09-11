package storage

import (
	"log/slog"
	"os"

	"github.com/laraxpy/photo-bucket-backend/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func Connect(cfg *config.Config) *minio.Client {
	client, err := minio.New(cfg.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinioAccessKey, cfg.MinioSecretKey, ""),
		Secure: false,
	})
	if err != nil {
		slog.Error("cannot connect to minio", "error", err)
		os.Exit(1)
	}
	return client
}
