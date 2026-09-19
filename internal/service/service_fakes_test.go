package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
	"github.com/laraxpy/photo-bucket-backend/internal/model/file"
	"github.com/laraxpy/photo-bucket-backend/internal/model/folder"
	"github.com/laraxpy/photo-bucket-backend/internal/model/token"
)

// fakeFolderStore and fakeFileStore are in-memory stand-ins for store.FolderStore
// and store.FileStore, used to unit-test the service layer's business rules
// (ownership, cycle prevention, duplicate names) without a real database.

type fakeFolderStore struct {
	folders map[uuid.UUID]folder.Folder
}

func newFakeFolderStore() *fakeFolderStore {
	return &fakeFolderStore{folders: make(map[uuid.UUID]folder.Folder)}
}

func (s *fakeFolderStore) Create(ctx context.Context, f *folder.Folder) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	s.folders[f.ID] = *f
	return nil
}

func (s *fakeFolderStore) GetByID(ctx context.Context, id uuid.UUID) (*folder.Folder, error) {
	f, ok := s.folders[id]
	if !ok {
		return nil, apperror.NotFound("folder not found", nil)
	}
	return &f, nil
}

func (s *fakeFolderStore) ListByUserID(ctx context.Context, userID uuid.UUID, parentID *uuid.UUID) ([]folder.Folder, error) {
	var result []folder.Folder
	for _, f := range s.folders {
		if f.UserID != userID {
			continue
		}
		if !sameUUIDPtr(f.ParentID, parentID) {
			continue
		}
		result = append(result, f)
	}
	return result, nil
}

func (s *fakeFolderStore) ExistsByUserParentAndName(ctx context.Context, userID uuid.UUID, parentID *uuid.UUID, name string) (bool, error) {
	for _, f := range s.folders {
		if f.UserID == userID && sameUUIDPtr(f.ParentID, parentID) && f.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (s *fakeFolderStore) HasChildren(ctx context.Context, id uuid.UUID) (bool, error) {
	for _, f := range s.folders {
		if f.ParentID != nil && *f.ParentID == id {
			return true, nil
		}
	}
	return false, nil
}

func (s *fakeFolderStore) Update(ctx context.Context, f *folder.Folder) error {
	if _, ok := s.folders[f.ID]; !ok {
		return apperror.NotFound("folder not found", nil)
	}
	s.folders[f.ID] = *f
	return nil
}

func (s *fakeFolderStore) Delete(ctx context.Context, id uuid.UUID) error {
	if _, ok := s.folders[id]; !ok {
		return apperror.NotFound("folder not found", nil)
	}
	delete(s.folders, id)
	return nil
}

func sameUUIDPtr(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

type fakeFileStore struct {
	files map[uuid.UUID]file.File
}

func newFakeFileStore() *fakeFileStore {
	return &fakeFileStore{files: make(map[uuid.UUID]file.File)}
}

func (s *fakeFileStore) Create(ctx context.Context, f *file.File) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	s.files[f.ID] = *f
	return nil
}

func (s *fakeFileStore) GetByID(ctx context.Context, id uuid.UUID) (*file.File, error) {
	f, ok := s.files[id]
	if !ok {
		return nil, apperror.NotFound("file not found", nil)
	}
	return &f, nil
}

func (s *fakeFileStore) GetByObjectKey(ctx context.Context, objectKey string) (*file.File, error) {
	for _, f := range s.files {
		if f.ObjectKey == objectKey {
			return &f, nil
		}
	}
	return nil, apperror.NotFound("file not found", nil)
}

func (s *fakeFileStore) ListByUserID(ctx context.Context, userID uuid.UUID, folderID *uuid.UUID, limit, offset int) ([]file.File, error) {
	var result []file.File
	for _, f := range s.files {
		if f.UserID != userID {
			continue
		}
		if folderID != nil && !sameUUIDPtr(f.FolderID, folderID) {
			continue
		}
		result = append(result, f)
	}
	return result, nil
}

func (s *fakeFileStore) Update(ctx context.Context, f *file.File) error {
	if _, ok := s.files[f.ID]; !ok {
		return apperror.NotFound("file not found", nil)
	}
	s.files[f.ID] = *f
	return nil
}

func (s *fakeFileStore) UpdateStatus(ctx context.Context, id uuid.UUID, status file.FileStatus) error {
	f, ok := s.files[id]
	if !ok {
		return apperror.NotFound("file not found", nil)
	}
	f.Status = status
	s.files[id] = f
	return nil
}

func (s *fakeFileStore) Delete(ctx context.Context, id uuid.UUID) error {
	if _, ok := s.files[id]; !ok {
		return apperror.NotFound("file not found", nil)
	}
	delete(s.files, id)
	return nil
}

func (s *fakeFileStore) HardDelete(ctx context.Context, id uuid.UUID) error {
	return s.Delete(ctx, id)
}

func (s *fakeFileStore) CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	for _, f := range s.files {
		if f.UserID == userID {
			count++
		}
	}
	return count, nil
}

func (s *fakeFileStore) CountByFolderID(ctx context.Context, folderID uuid.UUID) (int64, error) {
	var count int64
	for _, f := range s.files {
		if f.FolderID != nil && *f.FolderID == folderID {
			count++
		}
	}
	return count, nil
}

type fakeRefreshTokenStore struct {
	tokens map[uuid.UUID]token.RefreshToken
	byHash map[string]uuid.UUID
}

func newFakeRefreshTokenStore() *fakeRefreshTokenStore {
	return &fakeRefreshTokenStore{
		tokens: make(map[uuid.UUID]token.RefreshToken),
		byHash: make(map[string]uuid.UUID),
	}
}

func (s *fakeRefreshTokenStore) Create(ctx context.Context, t *token.RefreshToken) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	s.tokens[t.ID] = *t
	s.byHash[t.TokenHash] = t.ID
	return nil
}

func (s *fakeRefreshTokenStore) GetByTokenHash(ctx context.Context, tokenHash string) (*token.RefreshToken, error) {
	id, ok := s.byHash[tokenHash]
	if !ok {
		return nil, apperror.Unauthorized("invalid refresh token", nil)
	}
	t := s.tokens[id]
	return &t, nil
}

func (s *fakeRefreshTokenStore) Revoke(ctx context.Context, id string) error {
	tokenID, err := uuid.Parse(id)
	if err != nil {
		return apperror.Internal(err)
	}
	t, ok := s.tokens[tokenID]
	if !ok {
		return apperror.NotFound("refresh token not found", nil)
	}
	now := time.Now()
	t.RevokedAt = &now
	s.tokens[tokenID] = t
	return nil
}
