package service

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
	"github.com/laraxpy/photo-bucket-backend/internal/model/folder"
)

func TestFileService_ResolveFilesForZip(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	t.Run("combines fileIds and folderId, deduplicating", func(t *testing.T) {
		svc, _, folders, _ := newTestFileService()
		folderID := uuid.New()
		folders.folders[folderID] = folder.Folder{ID: folderID, UserID: userID, Name: "Viajes"}

		inFolder, err := svc.Upload(ctx, userID, &folderID, strings.NewReader("a"), "a.txt", "text/plain", 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		root, err := svc.Upload(ctx, userID, nil, strings.NewReader("b"), "b.txt", "text/plain", 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Ask for the folder AND explicitly for the file already in it —
		// it must appear once, not twice.
		files, err := svc.ResolveFilesForZip(ctx, userID, []uuid.UUID{inFolder.ID, root.ID}, &folderID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 2 {
			t.Fatalf("expected 2 unique files, got %d", len(files))
		}
	})

	t.Run("fails clean if a fileId is not owned by the user", func(t *testing.T) {
		svc, _, _, _ := newTestFileService()
		f, err := svc.Upload(ctx, userID, nil, strings.NewReader("a"), "a.txt", "text/plain", 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		intruder := uuid.New()
		_, err = svc.ResolveFilesForZip(ctx, intruder, []uuid.UUID{f.ID}, nil)
		appErr := apperror.From(err)
		if appErr.Code != apperror.CodeNotFound {
			t.Errorf("Code = %v, want %v", appErr.Code, apperror.CodeNotFound)
		}
	})

	t.Run("fails with bad request when nothing is selected", func(t *testing.T) {
		svc, _, _, _ := newTestFileService()

		_, err := svc.ResolveFilesForZip(ctx, userID, nil, nil)
		appErr := apperror.From(err)
		if appErr.Code != apperror.CodeBadRequest {
			t.Errorf("Code = %v, want %v", appErr.Code, apperror.CodeBadRequest)
		}
	})
}

func readZipEntries(t *testing.T, data []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("result is not a valid zip: %v", err)
	}
	entries := make(map[string]string)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("failed to open zip entry %q: %v", f.Name, err)
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("failed to read zip entry %q: %v", f.Name, err)
		}
		entries[f.Name] = string(content)
	}
	return entries
}

func TestFileService_StreamZip(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	t.Run("writes all files into a valid zip with correct contents", func(t *testing.T) {
		svc, _, _, _ := newTestFileService()
		a, _ := svc.Upload(ctx, userID, nil, strings.NewReader("contenido A"), "a.txt", "text/plain", 11)
		b, _ := svc.Upload(ctx, userID, nil, strings.NewReader("contenido B"), "b.txt", "text/plain", 11)

		files, err := svc.ResolveFilesForZip(ctx, userID, []uuid.UUID{a.ID, b.ID}, nil)
		if err != nil {
			t.Fatalf("unexpected error resolving: %v", err)
		}

		var buf bytes.Buffer
		if err := svc.StreamZip(ctx, files, &buf); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		entries := readZipEntries(t, buf.Bytes())
		if entries["a.txt"] != "contenido A" {
			t.Errorf("a.txt content = %q, want %q", entries["a.txt"], "contenido A")
		}
		if entries["b.txt"] != "contenido B" {
			t.Errorf("b.txt content = %q, want %q", entries["b.txt"], "contenido B")
		}
	})

	t.Run("disambiguates duplicate original names", func(t *testing.T) {
		svc, _, folders, _ := newTestFileService()
		folderID := uuid.New()
		folders.folders[folderID] = folder.Folder{ID: folderID, UserID: userID, Name: "Otra"}

		f1, _ := svc.Upload(ctx, userID, nil, strings.NewReader("uno"), "photo.jpg", "text/plain", 3)
		f2, _ := svc.Upload(ctx, userID, &folderID, strings.NewReader("dos"), "photo.jpg", "text/plain", 3)

		files, err := svc.ResolveFilesForZip(ctx, userID, []uuid.UUID{f1.ID, f2.ID}, nil)
		if err != nil {
			t.Fatalf("unexpected error resolving: %v", err)
		}

		var buf bytes.Buffer
		if err := svc.StreamZip(ctx, files, &buf); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		entries := readZipEntries(t, buf.Bytes())
		if len(entries) != 2 {
			t.Fatalf("expected 2 distinct zip entries, got %d: %v", len(entries), entries)
		}
		if entries["photo.jpg"] != "uno" {
			t.Errorf("photo.jpg content = %q, want %q", entries["photo.jpg"], "uno")
		}
		if entries["photo (1).jpg"] != "dos" {
			t.Errorf("photo (1).jpg content = %q, want %q; entries=%v", entries["photo (1).jpg"], "dos", entries)
		}
	})

	t.Run("bounds concurrency regardless of how many files are requested", func(t *testing.T) {
		svc, _, _, minioClient := newTestFileService()
		minioClient.getObjectDelay = 20 * time.Millisecond

		const fileCount = zipDownloadConcurrency * 3
		ids := make([]uuid.UUID, 0, fileCount)
		for i := 0; i < fileCount; i++ {
			f, err := svc.Upload(ctx, userID, nil, strings.NewReader("x"), "f.txt", "text/plain", 1)
			if err != nil {
				t.Fatalf("unexpected error uploading: %v", err)
			}
			ids = append(ids, f.ID)
		}

		files, err := svc.ResolveFilesForZip(ctx, userID, ids, nil)
		if err != nil {
			t.Fatalf("unexpected error resolving: %v", err)
		}

		var buf bytes.Buffer
		if err := svc.StreamZip(ctx, files, &buf); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if minioClient.getObjectCalls != fileCount {
			t.Errorf("getObjectCalls = %d, want %d", minioClient.getObjectCalls, fileCount)
		}
		if minioClient.maxActiveGet > zipDownloadConcurrency {
			t.Errorf("maxActiveGet = %d, want at most %d (concurrency was not bounded)", minioClient.maxActiveGet, zipDownloadConcurrency)
		}
		if minioClient.maxActiveGet < 2 {
			t.Errorf("maxActiveGet = %d, want at least 2 (test didn't actually exercise concurrency)", minioClient.maxActiveGet)
		}
	})

	t.Run("fails and does not hang if one file fails mid-stream", func(t *testing.T) {
		svc, _, _, minioClient := newTestFileService()
		a, _ := svc.Upload(ctx, userID, nil, strings.NewReader("uno"), "a.txt", "text/plain", 3)
		b, _ := svc.Upload(ctx, userID, nil, strings.NewReader("dos"), "b.txt", "text/plain", 3)
		c, _ := svc.Upload(ctx, userID, nil, strings.NewReader("tres"), "c.txt", "text/plain", 4)

		minioClient.getObjectErrForKey = b.ObjectKey

		files, err := svc.ResolveFilesForZip(ctx, userID, []uuid.UUID{a.ID, b.ID, c.ID}, nil)
		if err != nil {
			t.Fatalf("unexpected error resolving: %v", err)
		}

		done := make(chan error, 1)
		go func() {
			var buf bytes.Buffer
			done <- svc.StreamZip(ctx, files, &buf)
		}()

		select {
		case err := <-done:
			if err == nil {
				t.Fatal("expected an error when one file fails mid-stream")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("StreamZip hung instead of failing (likely deadlock)")
		}
	})
}
