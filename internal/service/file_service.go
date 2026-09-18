package service

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
	"github.com/laraxpy/photo-bucket-backend/internal/model/file"
	"github.com/laraxpy/photo-bucket-backend/internal/store"
	"github.com/minio/minio-go/v7"
)

const downloadURLExpiry = 15 * time.Minute

// MinioClient is the subset of *minio.Client that FileService depends on.
// Defining it here (instead of taking *minio.Client directly) lets tests
// fake the MinIO interaction without a real server.
type MinioClient interface {
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	PresignedGetObject(ctx context.Context, bucketName, objectName string, expiry time.Duration, reqParams url.Values) (*url.URL, error)
	RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error
}

type FileService interface {
	Upload(ctx context.Context, userId uuid.UUID, folderID *uuid.UUID, reader io.Reader, originalName, contentType string, size int64) (*file.File, error)
	ListByUser(ctx context.Context, userID uuid.UUID, folderID *uuid.UUID, limit, offset int) ([]file.File, error)
	GetByID(ctx context.Context, userID, id uuid.UUID) (*file.File, error)
	DownloadURL(ctx context.Context, userID, id uuid.UUID) (string, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
}
type fileService struct {
	fileStore   store.FileStore
	folderStore store.FolderStore
	minio       MinioClient
	bucketName  string
}

func (s *fileService) ListByUser(ctx context.Context, userID uuid.UUID, folderID *uuid.UUID, limit int, offset int) ([]file.File, error) {
	return s.fileStore.ListByUserID(ctx, userID, folderID, limit, offset)
}

func (s *fileService) Upload(ctx context.Context, userId uuid.UUID, folderID *uuid.UUID, reader io.Reader, originalName string, contentType string, size int64) (*file.File, error) {
	if folderID != nil {
		targetFolder, err := s.folderStore.GetByID(ctx, *folderID)
		if err != nil {
			return nil, err
		}
		if targetFolder.UserID != userId {
			return nil, apperror.NotFound("folder not found", nil)
		}
	}

	objectKey := fmt.Sprintf("%s/%s", userId.String(), uuid.New().String())
	_, err := s.minio.PutObject(ctx, s.bucketName, objectKey, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	f := &file.File{
		UserID:       userId,
		FolderID:     folderID,
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

func (s *fileService) GetByID(ctx context.Context, userID, id uuid.UUID) (*file.File, error) {
	f, err := s.fileStore.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if f.UserID != userID {
		return nil, apperror.NotFound("file not found", nil)
	}
	return f, nil
}

func (s *fileService) DownloadURL(ctx context.Context, userID, id uuid.UUID) (string, error) {
	f, err := s.GetByID(ctx, userID, id)
	if err != nil {
		return "", err
	}

	presignedURL, err := s.minio.PresignedGetObject(ctx, f.BucketName, f.ObjectKey, downloadURLExpiry, url.Values{})
	if err != nil {
		return "", apperror.Internal(err)
	}
	return presignedURL.String(), nil
}

func (s *fileService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	f, err := s.GetByID(ctx, userID, id)
	if err != nil {
		return err
	}

	if err := s.minio.RemoveObject(ctx, f.BucketName, f.ObjectKey, minio.RemoveObjectOptions{}); err != nil {
		return apperror.Internal(err)
	}

	return s.fileStore.Delete(ctx, f.ID)
}

func NewFileService(fileStore store.FileStore, folderStore store.FolderStore, minioClient MinioClient, bucketName string) FileService {
	return &fileService{fileStore: fileStore, folderStore: folderStore, minio: minioClient, bucketName: bucketName}
}
