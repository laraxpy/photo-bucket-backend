package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
	"github.com/laraxpy/photo-bucket-backend/internal/model/folder"
	"github.com/laraxpy/photo-bucket-backend/internal/store"
)

const maxFolderDepth = 100

type FolderService interface {
	Create(ctx context.Context, userID uuid.UUID, name, parentIDStr string) (*folder.Folder, error)
	GetByID(ctx context.Context, userID, id uuid.UUID) (*folder.Folder, error)
	ListByUser(ctx context.Context, userID uuid.UUID, parentIDStr string) ([]folder.Folder, error)
	Rename(ctx context.Context, userID, id uuid.UUID, name string) (*folder.Folder, error)
	Move(ctx context.Context, userID, id uuid.UUID, parentIDStr string) (*folder.Folder, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
}

type folderService struct {
	folderStore store.FolderStore
	fileStore   store.FileStore
}

func NewFolderService(folderStore store.FolderStore, fileStore store.FileStore) FolderService {
	return &folderService{folderStore: folderStore, fileStore: fileStore}
}

func parseOptionalUUID(raw string) (*uuid.UUID, error) {
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, apperror.BadRequest("invalid parentId", err)
	}
	return &id, nil
}

func sameParent(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func (s *folderService) getOwned(ctx context.Context, userID, id uuid.UUID) (*folder.Folder, error) {
	f, err := s.folderStore.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if f.UserID != userID {
		return nil, apperror.NotFound("folder not found", nil)
	}
	return f, nil
}

func (s *folderService) Create(ctx context.Context, userID uuid.UUID, name, parentIDStr string) (*folder.Folder, error) {
	parentID, err := parseOptionalUUID(parentIDStr)
	if err != nil {
		return nil, err
	}
	if parentID != nil {
		if _, err := s.getOwned(ctx, userID, *parentID); err != nil {
			return nil, err
		}
	}

	exists, err := s.folderStore.ExistsByUserParentAndName(ctx, userID, parentID, name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperror.Conflict("a folder with this name already exists here", nil)
	}

	f := &folder.Folder{UserID: userID, ParentID: parentID, Name: name}
	if err := s.folderStore.Create(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

func (s *folderService) GetByID(ctx context.Context, userID, id uuid.UUID) (*folder.Folder, error) {
	return s.getOwned(ctx, userID, id)
}

func (s *folderService) ListByUser(ctx context.Context, userID uuid.UUID, parentIDStr string) ([]folder.Folder, error) {
	parentID, err := parseOptionalUUID(parentIDStr)
	if err != nil {
		return nil, err
	}
	if parentID != nil {
		if _, err := s.getOwned(ctx, userID, *parentID); err != nil {
			return nil, err
		}
	}
	return s.folderStore.ListByUserID(ctx, userID, parentID)
}

func (s *folderService) Rename(ctx context.Context, userID, id uuid.UUID, name string) (*folder.Folder, error) {
	f, err := s.getOwned(ctx, userID, id)
	if err != nil {
		return nil, err
	}

	if !strings.EqualFold(f.Name, name) {
		exists, err := s.folderStore.ExistsByUserParentAndName(ctx, userID, f.ParentID, name)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, apperror.Conflict("a folder with this name already exists here", nil)
		}
	}

	f.Name = name
	if err := s.folderStore.Update(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

func (s *folderService) Move(ctx context.Context, userID, id uuid.UUID, parentIDStr string) (*folder.Folder, error) {
	f, err := s.getOwned(ctx, userID, id)
	if err != nil {
		return nil, err
	}

	newParentID, err := parseOptionalUUID(parentIDStr)
	if err != nil {
		return nil, err
	}

	if newParentID != nil {
		if *newParentID == id {
			return nil, apperror.BadRequest("a folder cannot be its own parent", nil)
		}
		parent, err := s.getOwned(ctx, userID, *newParentID)
		if err != nil {
			return nil, err
		}
		if err := s.assertNotDescendant(ctx, id, parent); err != nil {
			return nil, err
		}
	}

	if sameParent(f.ParentID, newParentID) {
		return f, nil
	}

	exists, err := s.folderStore.ExistsByUserParentAndName(ctx, userID, newParentID, f.Name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperror.Conflict("a folder with this name already exists here", nil)
	}

	f.ParentID = newParentID
	if err := s.folderStore.Update(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

// assertNotDescendant evita crear un ciclo en el arbol: candidate (el nuevo padre)
// no puede ser folderID ni descender de folderID.
func (s *folderService) assertNotDescendant(ctx context.Context, folderID uuid.UUID, candidate *folder.Folder) error {
	current := candidate
	for i := 0; i < maxFolderDepth; i++ {
		if current.ID == folderID {
			return apperror.BadRequest("cannot move a folder into one of its own subfolders", nil)
		}
		if current.ParentID == nil {
			return nil
		}
		parent, err := s.folderStore.GetByID(ctx, *current.ParentID)
		if err != nil {
			return err
		}
		current = parent
	}
	return apperror.Internal(errors.New("folder hierarchy too deep"))
}

func (s *folderService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	f, err := s.getOwned(ctx, userID, id)
	if err != nil {
		return err
	}

	hasChildren, err := s.folderStore.HasChildren(ctx, f.ID)
	if err != nil {
		return err
	}
	if hasChildren {
		return apperror.Conflict("folder is not empty: delete subfolders first", nil)
	}

	fileCount, err := s.fileStore.CountByFolderID(ctx, f.ID)
	if err != nil {
		return err
	}
	if fileCount > 0 {
		return apperror.Conflict("folder is not empty: move or delete its files first", nil)
	}

	return s.folderStore.Delete(ctx, f.ID)
}
