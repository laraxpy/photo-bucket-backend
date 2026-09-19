package storage

import (
	"context"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/laraxpy/photo-bucket-backend/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Client wraps *minio.Client to adapt GetObject's return type from the
// concrete *minio.Object to the generic io.ReadCloser. minio.Object has no
// exported constructor, so service.MinioClient (used by tests to fake MinIO
// without a real server) declares GetObject returning io.ReadCloser instead;
// this wrapper is what makes the real client satisfy that interface.
type Client struct {
	*minio.Client
}

func (c *Client) GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (io.ReadCloser, error) {
	return c.Client.GetObject(ctx, bucketName, objectName, opts)
}

func Connect(cfg *config.Config) *Client {
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

	return &Client{Client: client}
}
