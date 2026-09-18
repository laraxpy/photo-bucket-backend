package storage

import (
	"context"
	"log/slog"
	"os"
	"time"

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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := client.BucketExists(ctx, cfg.MinioBucket)
	if err != nil {
		slog.Error("cannot check minio bucket", "bucket", cfg.MinioBucket, "error", err)
		os.Exit(1)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.MinioBucket, minio.MakeBucketOptions{}); err != nil {
			slog.Error("cannot create minio bucket", "bucket", cfg.MinioBucket, "error", err)
			os.Exit(1)
		}
		slog.Info("minio bucket created", "bucket", cfg.MinioBucket)
	}

	return client
}
