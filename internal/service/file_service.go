package service

import (
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
	"github.com/laraxpy/photo-bucket-backend/internal/model/file"
	"github.com/laraxpy/photo-bucket-backend/internal/store"
	"github.com/minio/minio-go/v7"
)

type FileService interface {
	Upload(ctx context.Context, userId uuid.UUID, reader io.Reader, originalName, contentType string, size int64) (*file.File, error)
	ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]file.File, error)
}
type fileService struct {
	fileStore  store.FileStore
	minio      *minio.Client
	bucketName string
}

func (s *fileService) ListByUser(ctx context.Context, userID uuid.UUID, limit int, offset int) ([]file.File, error) {
	return s.fileStore.ListByUserID(ctx, userID, limit, offset)
}

func (s *fileService) Upload(ctx context.Context, userId uuid.UUID, reader io.Reader, originalName string, contentType string, size int64) (*file.File, error) {
	objectKey := fmt.Sprintf("%s/%s", userId.String(), uuid.New().String())
	_, err := s.minio.PutObject(ctx, s.bucketName, objectKey, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	f := &file.File{
		UserID:       userId,
		BucketName:   s.bucketName,
		ObjectKey:    objectKey,
		OriginalName: originalName,
		ContentType:  contentType,
		SizeBytes:    size,
		Status:       file.FileStatusUploaded,
	}
	if err := s.fileStore.Create(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

func NewFileService(fileStore store.FileStore, minioClient *minio.Client, bucketName string) FileService {
	return &fileService{fileStore: fileStore, minio: minioClient, bucketName: bucketName}
}
