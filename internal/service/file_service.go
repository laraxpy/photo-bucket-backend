package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
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
	ThumbnailURL(ctx context.Context, userID, id uuid.UUID) (string, error)
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

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	objectKey := fmt.Sprintf("%s/%s", userId.String(), uuid.New().String())
	_, err = s.minio.PutObject(ctx, s.bucketName, objectKey, bytes.NewReader(data), size, minio.PutObjectOptions{
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

	thumbnailKey, err := s.uploadThumbnail(ctx, objectKey, data, contentType)
	switch {
	case err == nil:
		f.ThumbnailObjectKey = thumbnailKey
	case errors.Is(err, errThumbnailUnsupportedType):
		// expected: not every content type has a thumbnail generator
	default:
		slog.Warn("no se pudo generar la miniatura", "objectKey", objectKey, "contentType", contentType, "error", err)
	}

	if err := s.fileStore.Create(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

// uploadThumbnail generates and uploads a thumbnail for supported image
// types, returning its object key. Returns errThumbnailUnsupportedType
// (unwrapped by the caller via errors.Is) for content types we don't
// generate thumbnails for; that's an expected skip, not a failure.
func (s *fileService) uploadThumbnail(ctx context.Context, objectKey string, data []byte, contentType string) (string, error) {
	thumbData, err := generateThumbnail(data, contentType)
	if err != nil {
		return "", err
	}

	thumbnailKey := "thumbnails/" + objectKey + ".jpg"
	_, err = s.minio.PutObject(ctx, s.bucketName, thumbnailKey, bytes.NewReader(thumbData), int64(len(thumbData)), minio.PutObjectOptions{
		ContentType: thumbnailContentType,
	})
	if err != nil {
		return "", err
	}
	return thumbnailKey, nil
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

// ThumbnailURL returns a presigned URL for the file's thumbnail. Files
// without a thumbnail (unsupported content type, or generation failed at
// upload time) fall back to the original, so the frontend always gets a
// usable preview URL.
func (s *fileService) ThumbnailURL(ctx context.Context, userID, id uuid.UUID) (string, error) {
	f, err := s.GetByID(ctx, userID, id)
	if err != nil {
		return "", err
	}

	key := f.ObjectKey
	if f.ThumbnailObjectKey != "" {
		key = f.ThumbnailObjectKey
	}

	presignedURL, err := s.minio.PresignedGetObject(ctx, f.BucketName, key, downloadURLExpiry, url.Values{})
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

	if f.ThumbnailObjectKey != "" {
		if err := s.minio.RemoveObject(ctx, f.BucketName, f.ThumbnailObjectKey, minio.RemoveObjectOptions{}); err != nil {
			slog.Warn("no se pudo borrar la miniatura", "thumbnailObjectKey", f.ThumbnailObjectKey, "error", err)
		}
	}

	return s.fileStore.Delete(ctx, f.ID)
}

func NewFileService(fileStore store.FileStore, folderStore store.FolderStore, minioClient MinioClient, bucketName string) FileService {
	return &fileService{fileStore: fileStore, folderStore: folderStore, minio: minioClient, bucketName: bucketName}
}
