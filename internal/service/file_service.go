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
	// GetObject returns io.ReadCloser rather than the SDK's concrete
	// *minio.Object (which has no exported constructor and so can't be
	// faked in tests) — see storage.Client for the real implementation's
	// adapter.
	GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (io.ReadCloser, error)
	PresignedGetObject(ctx context.Context, bucketName, objectName string, expiry time.Duration, reqParams url.Values) (*url.URL, error)
	RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error
}

// ThumbnailSize selects which thumbnail variant to resolve a URL for.
type ThumbnailSize string

const (
	ThumbnailSizeSmall  ThumbnailSize = "small"
	ThumbnailSizeMedium ThumbnailSize = "medium"
)

type FileService interface {
	Upload(ctx context.Context, userId uuid.UUID, folderID *uuid.UUID, reader io.Reader, originalName, contentType string, size int64) (*file.File, error)
	ListByUser(ctx context.Context, userID uuid.UUID, folderID *uuid.UUID, limit, offset int) ([]file.File, error)
	GetByID(ctx context.Context, userID, id uuid.UUID) (*file.File, error)
	DownloadURL(ctx context.Context, userID, id uuid.UUID) (string, error)
	ThumbnailURL(ctx context.Context, userID, id uuid.UUID, size ThumbnailSize) (string, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error

	// ResolveFilesForZip validates ownership of every requested file (by
	// explicit IDs and/or all files in a folder) before anything is
	// streamed. Callers must run this — and only commit to writing a
	// response — after it succeeds, so a bad ID fails clean with a JSON
	// error instead of mid-stream.
	ResolveFilesForZip(ctx context.Context, userID uuid.UUID, fileIDs []uuid.UUID, folderID *uuid.UUID) ([]file.File, error)
	// StreamZip downloads the given (already-validated) files with bounded
	// concurrency and writes them as a zip archive to w, in order.
	StreamZip(ctx context.Context, files []file.File, w io.Writer) error
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

	thumbnails, err := generateThumbnails(data, contentType)
	switch {
	case err == nil:
		if key, uploadErr := s.uploadThumbnailVariant(ctx, objectKey, "small", thumbnails.Small); uploadErr != nil {
			slog.Warn("no se pudo subir la miniatura pequeña", "objectKey", objectKey, "error", uploadErr)
		} else {
			f.ThumbnailSmallObjectKey = key
		}
		if key, uploadErr := s.uploadThumbnailVariant(ctx, objectKey, "medium", thumbnails.Medium); uploadErr != nil {
			slog.Warn("no se pudo subir la miniatura mediana", "objectKey", objectKey, "error", uploadErr)
		} else {
			f.ThumbnailMediumObjectKey = key
		}
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

// uploadThumbnailVariant uploads one already-generated thumbnail variant
// (e.g. "small" or "medium"), returning its object key.
func (s *fileService) uploadThumbnailVariant(ctx context.Context, objectKey, variant string, data []byte) (string, error) {
	key := fmt.Sprintf("thumbnails/%s/%s.jpg", variant, objectKey)
	_, err := s.minio.PutObject(ctx, s.bucketName, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: thumbnailContentType,
	})
	if err != nil {
		return "", err
	}
	return key, nil
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

// ThumbnailURL returns a presigned URL for the requested thumbnail variant.
// It falls back gracefully when a variant is missing (unsupported content
// type, or generation failed at upload time): medium falls back to small,
// and either falls back to the original — so the frontend always gets a
// usable preview URL regardless of size.
func (s *fileService) ThumbnailURL(ctx context.Context, userID, id uuid.UUID, size ThumbnailSize) (string, error) {
	f, err := s.GetByID(ctx, userID, id)
	if err != nil {
		return "", err
	}

	key := f.ObjectKey
	switch size {
	case ThumbnailSizeMedium:
		switch {
		case f.ThumbnailMediumObjectKey != "":
			key = f.ThumbnailMediumObjectKey
		case f.ThumbnailSmallObjectKey != "":
			key = f.ThumbnailSmallObjectKey
		}
	default:
		if f.ThumbnailSmallObjectKey != "" {
			key = f.ThumbnailSmallObjectKey
		}
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

	for _, thumbnailKey := range []string{f.ThumbnailSmallObjectKey, f.ThumbnailMediumObjectKey} {
		if thumbnailKey == "" {
			continue
		}
		if err := s.minio.RemoveObject(ctx, f.BucketName, thumbnailKey, minio.RemoveObjectOptions{}); err != nil {
			slog.Warn("no se pudo borrar la miniatura", "thumbnailObjectKey", thumbnailKey, "error", err)
		}
	}

	return s.fileStore.Delete(ctx, f.ID)
}

func NewFileService(fileStore store.FileStore, folderStore store.FolderStore, minioClient MinioClient, bucketName string) FileService {
	return &fileService{fileStore: fileStore, folderStore: folderStore, minio: minioClient, bucketName: bucketName}
}
