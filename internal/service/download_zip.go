package service

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
	"github.com/laraxpy/photo-bucket-backend/internal/model/file"
	"github.com/minio/minio-go/v7"
)

// zipDownloadConcurrency bounds how many files are downloaded from MinIO to
// temp files at once. It's a hard cap regardless of how many files were
// requested, so a bulk download of any size uses a predictable, small amount
// of memory/disk/MinIO connections instead of growing with the request.
const zipDownloadConcurrency = 4

// listAllLimit stands in for "no pagination limit" when listing every file
// in a folder for a zip download — the store only offers a paginated
// listing, and a bulk download intentionally has no size cap.
const listAllLimit = 1_000_000

// ResolveFilesForZip validates ownership of every requested file before any
// bytes are ever written, so a bad ID fails clean with a JSON error instead
// of mid-stream (once the zip starts, we can no longer send a normal HTTP
// error response).
func (s *fileService) ResolveFilesForZip(ctx context.Context, userID uuid.UUID, fileIDs []uuid.UUID, folderID *uuid.UUID) ([]file.File, error) {
	var files []file.File
	seen := make(map[uuid.UUID]bool)

	if folderID != nil {
		folderFiles, err := s.fileStore.ListByUserID(ctx, userID, folderID, listAllLimit, 0)
		if err != nil {
			return nil, err
		}
		for _, f := range folderFiles {
			if !seen[f.ID] {
				seen[f.ID] = true
				files = append(files, f)
			}
		}
	}

	for _, id := range fileIDs {
		if seen[id] {
			continue
		}
		f, err := s.GetByID(ctx, userID, id)
		if err != nil {
			return nil, err
		}
		seen[f.ID] = true
		files = append(files, *f)
	}

	if len(files) == 0 {
		return nil, apperror.BadRequest("no files selected for download", nil)
	}
	return files, nil
}

type zipDownloadResult struct {
	tempPath string
	err      error
}

// StreamZip downloads files with bounded concurrency and writes them to w as
// a zip archive, in the given order. At most zipDownloadConcurrency files
// are ever in flight (being downloaded, or downloaded but not yet written to
// the zip) at once — a slow/large file at the front doesn't let unbounded
// work pile up behind it, since a worker only starts its next download once
// the consumer has caught up enough to free a slot.
//
// Once this starts writing to w, a failure can no longer be reported as a
// clean HTTP error — the caller has already committed to a 200 response.
// StreamZip cancels remaining downloads and returns the error; the resulting
// zip is incomplete/truncated on the wire, which is the best that can be
// done at that point.
func (s *fileService) StreamZip(ctx context.Context, files []file.File, w io.Writer) error {
	entryNames := uniqueZipEntryNames(files)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([]chan zipDownloadResult, len(files))
	for i := range results {
		results[i] = make(chan zipDownloadResult, 1)
	}

	sem := make(chan struct{}, zipDownloadConcurrency)
	var wg sync.WaitGroup
	go func() {
		for i, f := range files {
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				// Every index must get a result no matter what, or the
				// consumer loop below blocks forever reading results[i].
				results[i] <- zipDownloadResult{err: ctx.Err()}
				continue
			}
			wg.Add(1)
			go func(i int, f file.File) {
				defer wg.Done()
				tempPath, err := s.downloadToTempFile(ctx, f)
				results[i] <- zipDownloadResult{tempPath: tempPath, err: err}
			}(i, f)
		}
	}()

	zw := zip.NewWriter(w)
	var firstErr error
	for i := range files {
		res := <-results[i]
		if res.tempPath != "" {
			if firstErr == nil {
				if err := appendTempFileToZip(zw, res.tempPath, entryNames[i]); err != nil {
					firstErr = err
					cancel()
				}
			}
			os.Remove(res.tempPath)
		}
		if res.err != nil && firstErr == nil {
			firstErr = res.err
			cancel()
		}
		<-sem
	}
	wg.Wait()

	if firstErr != nil {
		zw.Close()
		slog.Error("fallo generando el zip de descarga masiva", "error", firstErr)
		return apperror.Internal(firstErr)
	}
	return zw.Close()
}

func (s *fileService) downloadToTempFile(ctx context.Context, f file.File) (string, error) {
	obj, err := s.minio.GetObject(ctx, f.BucketName, f.ObjectKey, minio.GetObjectOptions{})
	if err != nil {
		return "", err
	}
	defer obj.Close()

	tmp, err := os.CreateTemp("", "zip-dl-*")
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	if _, err := io.Copy(tmp, obj); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}

func appendTempFileToZip(zw *zip.Writer, tempPath, entryName string) error {
	f, err := os.Open(tempPath)
	if err != nil {
		return err
	}
	defer f.Close()

	entry, err := zw.Create(entryName)
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, f)
	return err
}

// uniqueZipEntryNames returns a zip entry name per file, using the original
// filename but disambiguating duplicates (e.g. the same "IMG001.jpg" name
// uploaded to two different folders) so nothing gets silently overwritten
// inside the archive.
func uniqueZipEntryNames(files []file.File) []string {
	counts := make(map[string]int)
	names := make([]string, len(files))
	for i, f := range files {
		name := f.OriginalName
		if name == "" {
			name = f.ID.String()
		}
		counts[name]++
		if n := counts[name]; n > 1 {
			ext := filepath.Ext(name)
			base := strings.TrimSuffix(name, ext)
			name = fmt.Sprintf("%s (%d)%s", base, n-1, ext)
		}
		names[i] = name
	}
	return names
}
