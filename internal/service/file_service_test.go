package service

import (
	"context"
	"errors"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
	"github.com/laraxpy/photo-bucket-backend/internal/model/folder"
	"github.com/minio/minio-go/v7"
)

// fakeMinioClient records calls and lets tests control failures, without
// touching a real MinIO server. It satisfies the MinioClient interface.
type fakeMinioClient struct {
	putObjectErr    error
	presignedErr    error
	removeObjectErr error

	putObjectCalls    int
	presignedCalls    int
	removeObjectCalls int
	lastRemovedKey    string
}

func (m *fakeMinioClient) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	m.putObjectCalls++
	if m.putObjectErr != nil {
		return minio.UploadInfo{}, m.putObjectErr
	}
	return minio.UploadInfo{Bucket: bucketName, Key: objectName}, nil
}

func (m *fakeMinioClient) PresignedGetObject(ctx context.Context, bucketName, objectName string, expiry time.Duration, reqParams url.Values) (*url.URL, error) {
	m.presignedCalls++
	if m.presignedErr != nil {
		return nil, m.presignedErr
	}
	return url.Parse("http://localhost:9000/" + bucketName + "/" + objectName + "?signed=1")
}

func (m *fakeMinioClient) RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
	m.removeObjectCalls++
	m.lastRemovedKey = objectName
	return m.removeObjectErr
}

func newTestFileService() (FileService, *fakeFileStore, *fakeFolderStore, *fakeMinioClient) {
	files := newFakeFileStore()
	folders := newFakeFolderStore()
	minioClient := &fakeMinioClient{}
	svc := NewFileService(files, folders, minioClient, "test-bucket")
	return svc, files, folders, minioClient
}

func TestFileService_Upload(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	t.Run("uploads to root when no folder given", func(t *testing.T) {
		svc, _, _, minioClient := newTestFileService()
		f, err := svc.Upload(ctx, userID, nil, strings.NewReader("hola"), "a.txt", "text/plain", 4)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.FolderID != nil {
			t.Errorf("expected nil FolderID, got %v", f.FolderID)
		}
		if minioClient.putObjectCalls != 1 {
			t.Errorf("PutObject calls = %d, want 1", minioClient.putObjectCalls)
		}
	})

	t.Run("rejects a folder owned by another user", func(t *testing.T) {
		svc, _, folders, _ := newTestFileService()
		otherUser := uuid.New()
		otherFolder := uuid.New()
		folders.folders[otherFolder] = folder.Folder{ID: otherFolder, UserID: otherUser, Name: "Privado"}

		_, err := svc.Upload(ctx, userID, &otherFolder, strings.NewReader("x"), "a.txt", "text/plain", 1)
		if err == nil {
			t.Fatal("expected an error uploading into another user's folder")
		}
		appErr := apperror.From(err)
		if appErr.Code != apperror.CodeNotFound {
			t.Errorf("Code = %v, want %v", appErr.Code, apperror.CodeNotFound)
		}
	})

	t.Run("does not create a DB row when PutObject fails", func(t *testing.T) {
		svc, files, _, minioClient := newTestFileService()
		minioClient.putObjectErr = errors.New("minio is down")

		_, err := svc.Upload(ctx, userID, nil, strings.NewReader("x"), "a.txt", "text/plain", 1)
		if err == nil {
			t.Fatal("expected an error when PutObject fails")
		}
		if len(files.files) != 0 {
			t.Errorf("expected no file rows to be created, got %d", len(files.files))
		}
	})
}

func TestFileService_ListByUser(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	t.Run("root listing excludes files inside folders", func(t *testing.T) {
		svc, _, folders, _ := newTestFileService()
		folderID := uuid.New()
		folders.folders[folderID] = folder.Folder{ID: folderID, UserID: userID, Name: "Viajes"}

		root, err := svc.Upload(ctx, userID, nil, strings.NewReader("root"), "root.txt", "text/plain", 4)
		if err != nil {
			t.Fatalf("unexpected error uploading to root: %v", err)
		}
		if _, err := svc.Upload(ctx, userID, &folderID, strings.NewReader("inside"), "inside.txt", "text/plain", 6); err != nil {
			t.Fatalf("unexpected error uploading to folder: %v", err)
		}

		files, err := svc.ListByUser(ctx, userID, nil, 20, 0)
		if err != nil {
			t.Fatalf("unexpected error listing: %v", err)
		}
		if len(files) != 1 || files[0].ID != root.ID {
			t.Errorf("expected only the root file %v, got %v", root.ID, files)
		}
	})

	t.Run("folder listing only returns files inside that folder", func(t *testing.T) {
		svc, _, folders, _ := newTestFileService()
		folderID := uuid.New()
		folders.folders[folderID] = folder.Folder{ID: folderID, UserID: userID, Name: "Viajes"}

		if _, err := svc.Upload(ctx, userID, nil, strings.NewReader("root"), "root.txt", "text/plain", 4); err != nil {
			t.Fatalf("unexpected error uploading to root: %v", err)
		}
		inside, err := svc.Upload(ctx, userID, &folderID, strings.NewReader("inside"), "inside.txt", "text/plain", 6)
		if err != nil {
			t.Fatalf("unexpected error uploading to folder: %v", err)
		}

		files, err := svc.ListByUser(ctx, userID, &folderID, 20, 0)
		if err != nil {
			t.Fatalf("unexpected error listing: %v", err)
		}
		if len(files) != 1 || files[0].ID != inside.ID {
			t.Errorf("expected only the folder file %v, got %v", inside.ID, files)
		}
	})
}

func TestFileService_DownloadURL(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	t.Run("returns a presigned URL for the owner", func(t *testing.T) {
		svc, files, _, _ := newTestFileService()
		f, _ := svc.Upload(ctx, userID, nil, strings.NewReader("hola"), "a.txt", "text/plain", 4)
		_ = files

		url, err := svc.DownloadURL(ctx, userID, f.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(url, "signed=1") {
			t.Errorf("expected a signed URL, got %q", url)
		}
	})

	t.Run("hides another user's file as 404", func(t *testing.T) {
		svc, _, _, _ := newTestFileService()
		f, _ := svc.Upload(ctx, userID, nil, strings.NewReader("hola"), "a.txt", "text/plain", 4)

		intruder := uuid.New()
		_, err := svc.DownloadURL(ctx, intruder, f.ID)
		appErr := apperror.From(err)
		if appErr.Code != apperror.CodeNotFound {
			t.Errorf("Code = %v, want %v", appErr.Code, apperror.CodeNotFound)
		}
	})
}

func TestFileService_Delete(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	t.Run("removes the MinIO object then the DB row", func(t *testing.T) {
		svc, files, _, minioClient := newTestFileService()
		f, _ := svc.Upload(ctx, userID, nil, strings.NewReader("hola"), "a.txt", "text/plain", 4)

		if err := svc.Delete(ctx, userID, f.ID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if minioClient.removeObjectCalls != 1 {
			t.Errorf("RemoveObject calls = %d, want 1", minioClient.removeObjectCalls)
		}
		if minioClient.lastRemovedKey != f.ObjectKey {
			t.Errorf("removed key = %q, want %q", minioClient.lastRemovedKey, f.ObjectKey)
		}
		if _, ok := files.files[f.ID]; ok {
			t.Error("expected the file row to be gone after delete")
		}
	})

	t.Run("keeps the DB row when RemoveObject fails", func(t *testing.T) {
		svc, files, _, minioClient := newTestFileService()
		f, _ := svc.Upload(ctx, userID, nil, strings.NewReader("hola"), "a.txt", "text/plain", 4)
		minioClient.removeObjectErr = errors.New("minio is down")

		err := svc.Delete(ctx, userID, f.ID)
		if err == nil {
			t.Fatal("expected an error when RemoveObject fails")
		}
		if _, ok := files.files[f.ID]; !ok {
			t.Error("expected the file row to survive a failed MinIO removal, to avoid orphaning the blob")
		}
	})

	t.Run("refuses to delete another user's file", func(t *testing.T) {
		svc, _, _, _ := newTestFileService()
		f, _ := svc.Upload(ctx, userID, nil, strings.NewReader("hola"), "a.txt", "text/plain", 4)

		intruder := uuid.New()
		err := svc.Delete(ctx, intruder, f.ID)
		appErr := apperror.From(err)
		if appErr.Code != apperror.CodeNotFound {
			t.Errorf("Code = %v, want %v", appErr.Code, apperror.CodeNotFound)
		}
	})
}
