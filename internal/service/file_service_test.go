package service

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
	"github.com/laraxpy/photo-bucket-backend/internal/model/file"
	"github.com/laraxpy/photo-bucket-backend/internal/model/folder"
	"github.com/minio/minio-go/v7"
)

// newTestJPEG builds a small, valid in-memory JPEG for tests that need a
// real decodable image (thumbnail generation reads actual pixel data).
func newTestJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("failed to encode test JPEG: %v", err)
	}
	return buf.Bytes()
}

// fakeMinioClient records calls and lets tests control failures, without
// touching a real MinIO server. It satisfies the MinioClient interface.
type fakeMinioClient struct {
	putObjectErr    error
	presignedErr    error
	removeObjectErr error

	putObjectCalls    int
	putObjectKeys     []string
	presignedCalls    int
	removeObjectCalls int
	removedKeys       []string
	lastRemovedKey    string
}

func (m *fakeMinioClient) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	m.putObjectCalls++
	m.putObjectKeys = append(m.putObjectKeys, objectName)
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
	m.removedKeys = append(m.removedKeys, objectName)
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

	t.Run("generates and uploads both thumbnail variants for a supported image type", func(t *testing.T) {
		svc, _, _, minioClient := newTestFileService()
		jpegData := newTestJPEG(t, 800, 600)

		f, err := svc.Upload(ctx, userID, nil, bytes.NewReader(jpegData), "photo.jpg", "image/jpeg", int64(len(jpegData)))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.ThumbnailSmallObjectKey == "" || f.ThumbnailMediumObjectKey == "" {
			t.Fatalf("expected both thumbnail object keys, got small=%q medium=%q", f.ThumbnailSmallObjectKey, f.ThumbnailMediumObjectKey)
		}
		if minioClient.putObjectCalls != 3 {
			t.Fatalf("PutObject calls = %d, want 3 (original + small + medium)", minioClient.putObjectCalls)
		}
		if !strings.Contains(f.ThumbnailSmallObjectKey, "thumbnails/small/") {
			t.Errorf("small thumbnail key = %q, want it under thumbnails/small/", f.ThumbnailSmallObjectKey)
		}
		if !strings.Contains(f.ThumbnailMediumObjectKey, "thumbnails/medium/") {
			t.Errorf("medium thumbnail key = %q, want it under thumbnails/medium/", f.ThumbnailMediumObjectKey)
		}
	})

	t.Run("does not generate a thumbnail for unsupported content types", func(t *testing.T) {
		svc, _, _, minioClient := newTestFileService()

		f, err := svc.Upload(ctx, userID, nil, strings.NewReader("%PDF-1.4 ..."), "doc.pdf", "application/pdf", 12)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.ThumbnailSmallObjectKey != "" || f.ThumbnailMediumObjectKey != "" {
			t.Errorf("expected no thumbnails, got small=%q medium=%q", f.ThumbnailSmallObjectKey, f.ThumbnailMediumObjectKey)
		}
		if minioClient.putObjectCalls != 1 {
			t.Errorf("PutObject calls = %d, want 1 (original only)", minioClient.putObjectCalls)
		}
	})

	t.Run("generates and uploads both thumbnail variants for a video", func(t *testing.T) {
		videoData := newTestMP4(t)
		svc, _, _, minioClient := newTestFileService()

		f, err := svc.Upload(ctx, userID, nil, bytes.NewReader(videoData), "clip.mp4", "video/mp4", int64(len(videoData)))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.ThumbnailSmallObjectKey == "" || f.ThumbnailMediumObjectKey == "" {
			t.Fatalf("expected both thumbnail object keys, got small=%q medium=%q", f.ThumbnailSmallObjectKey, f.ThumbnailMediumObjectKey)
		}
		if minioClient.putObjectCalls != 3 {
			t.Fatalf("PutObject calls = %d, want 3 (original + small + medium)", minioClient.putObjectCalls)
		}
	})

	t.Run("upload still succeeds when the image data can't be decoded", func(t *testing.T) {
		svc, files, _, minioClient := newTestFileService()

		f, err := svc.Upload(ctx, userID, nil, strings.NewReader("not-actually-a-jpeg"), "broken.jpg", "image/jpeg", 19)
		if err != nil {
			t.Fatalf("a broken image must not fail the whole upload: %v", err)
		}
		if f.ThumbnailSmallObjectKey != "" || f.ThumbnailMediumObjectKey != "" {
			t.Errorf("expected no thumbnails for undecodable data, got small=%q medium=%q", f.ThumbnailSmallObjectKey, f.ThumbnailMediumObjectKey)
		}
		if minioClient.putObjectCalls != 1 {
			t.Errorf("PutObject calls = %d, want 1 (original only)", minioClient.putObjectCalls)
		}
		if _, ok := files.files[f.ID]; !ok {
			t.Error("expected the file row to still be created")
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

func TestFileService_ThumbnailURL(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	t.Run("small returns the small thumbnail", func(t *testing.T) {
		svc, _, _, _ := newTestFileService()
		jpegData := newTestJPEG(t, 800, 600)
		f, err := svc.Upload(ctx, userID, nil, bytes.NewReader(jpegData), "photo.jpg", "image/jpeg", int64(len(jpegData)))
		if err != nil {
			t.Fatalf("unexpected error uploading: %v", err)
		}

		thumbURL, err := svc.ThumbnailURL(ctx, userID, f.ID, ThumbnailSizeSmall)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(thumbURL, f.ThumbnailSmallObjectKey) {
			t.Errorf("expected the small thumbnail URL, got %q", thumbURL)
		}
	})

	t.Run("medium returns the medium thumbnail", func(t *testing.T) {
		svc, _, _, _ := newTestFileService()
		jpegData := newTestJPEG(t, 800, 600)
		f, err := svc.Upload(ctx, userID, nil, bytes.NewReader(jpegData), "photo.jpg", "image/jpeg", int64(len(jpegData)))
		if err != nil {
			t.Fatalf("unexpected error uploading: %v", err)
		}

		thumbURL, err := svc.ThumbnailURL(ctx, userID, f.ID, ThumbnailSizeMedium)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(thumbURL, f.ThumbnailMediumObjectKey) {
			t.Errorf("expected the medium thumbnail URL, got %q", thumbURL)
		}
	})

	t.Run("falls back to the original when there are no thumbnails at all", func(t *testing.T) {
		svc, _, _, _ := newTestFileService()
		f, err := svc.Upload(ctx, userID, nil, strings.NewReader("%PDF-1.4 ..."), "doc.pdf", "application/pdf", 12)
		if err != nil {
			t.Fatalf("unexpected error uploading: %v", err)
		}

		thumbURL, err := svc.ThumbnailURL(ctx, userID, f.ID, ThumbnailSizeSmall)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(thumbURL, f.ObjectKey) {
			t.Errorf("expected a fallback URL for the original object %q, got %q", f.ObjectKey, thumbURL)
		}
	})

	t.Run("medium falls back to small when there is no medium", func(t *testing.T) {
		svc, files, _, _ := newTestFileService()
		id := uuid.New()
		files.files[id] = file.File{
			ID:                      id,
			UserID:                  userID,
			BucketName:              "test-bucket",
			ObjectKey:               "original-key",
			ThumbnailSmallObjectKey: "small-key",
		}

		thumbURL, err := svc.ThumbnailURL(ctx, userID, id, ThumbnailSizeMedium)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(thumbURL, "small-key") {
			t.Errorf("expected fallback to the small thumbnail, got %q", thumbURL)
		}
	})

	t.Run("hides another user's file as 404", func(t *testing.T) {
		svc, _, _, _ := newTestFileService()
		f, _ := svc.Upload(ctx, userID, nil, strings.NewReader("hola"), "a.txt", "text/plain", 4)

		intruder := uuid.New()
		_, err := svc.ThumbnailURL(ctx, intruder, f.ID, ThumbnailSizeSmall)
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

	t.Run("also removes both thumbnail objects when present", func(t *testing.T) {
		svc, _, _, minioClient := newTestFileService()
		jpegData := newTestJPEG(t, 800, 600)
		f, err := svc.Upload(ctx, userID, nil, bytes.NewReader(jpegData), "photo.jpg", "image/jpeg", int64(len(jpegData)))
		if err != nil {
			t.Fatalf("unexpected error uploading: %v", err)
		}

		if err := svc.Delete(ctx, userID, f.ID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if minioClient.removeObjectCalls != 3 {
			t.Fatalf("RemoveObject calls = %d, want 3 (original + small + medium)", minioClient.removeObjectCalls)
		}
		for _, wantKey := range []string{f.ThumbnailSmallObjectKey, f.ThumbnailMediumObjectKey} {
			found := false
			for _, key := range minioClient.removedKeys {
				if key == wantKey {
					found = true
				}
			}
			if !found {
				t.Errorf("expected the thumbnail key %q among removed keys %v", wantKey, minioClient.removedKeys)
			}
		}
	})
}
