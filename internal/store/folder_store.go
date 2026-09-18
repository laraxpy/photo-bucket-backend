package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
	"github.com/laraxpy/photo-bucket-backend/internal/model/folder"
	"gorm.io/gorm"
)

type FolderStore interface {
	Create(ctx context.Context, f *folder.Folder) error
	GetByID(ctx context.Context, id uuid.UUID) (*folder.Folder, error)
	ListByUserID(ctx context.Context, userID uuid.UUID, parentID *uuid.UUID) ([]folder.Folder, error)
	ExistsByUserParentAndName(ctx context.Context, userID uuid.UUID, parentID *uuid.UUID, name string) (bool, error)
	HasChildren(ctx context.Context, id uuid.UUID) (bool, error)
	Update(ctx context.Context, f *folder.Folder) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type gormFolderStore struct {
	db *gorm.DB
}

func NewFolderStore(db *gorm.DB) FolderStore {
	return &gormFolderStore{db: db}
}

func (s *gormFolderStore) Create(ctx context.Context, f *folder.Folder) error {
	if err := s.db.WithContext(ctx).Create(f).Error; err != nil {
		return apperror.Internal(err)
	}
	return nil
}

func (s *gormFolderStore) GetByID(ctx context.Context, id uuid.UUID) (*folder.Folder, error) {
	var f folder.Folder
	err := s.db.WithContext(ctx).First(&f, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperror.NotFound("folder not found", err)
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &f, nil
}

func (s *gormFolderStore) ListByUserID(ctx context.Context, userID uuid.UUID, parentID *uuid.UUID) ([]folder.Folder, error) {
	var folders []folder.Folder
	query := s.db.WithContext(ctx).Where("user_id = ?", userID)
	if parentID != nil {
		query = query.Where("parent_id = ?", *parentID)
	} else {
		query = query.Where("parent_id IS NULL")
	}
	if err := query.Order("name ASC").Find(&folders).Error; err != nil {
		return nil, apperror.Internal(err)
	}
	return folders, nil
}

func (s *gormFolderStore) ExistsByUserParentAndName(ctx context.Context, userID uuid.UUID, parentID *uuid.UUID, name string) (bool, error) {
	var count int64
	query := s.db.WithContext(ctx).Model(&folder.Folder{}).Where("user_id = ? AND name = ?", userID, name)
	if parentID != nil {
		query = query.Where("parent_id = ?", *parentID)
	} else {
		query = query.Where("parent_id IS NULL")
	}
	if err := query.Count(&count).Error; err != nil {
		return false, apperror.Internal(err)
	}
	return count > 0, nil
}

func (s *gormFolderStore) HasChildren(ctx context.Context, id uuid.UUID) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&folder.Folder{}).Where("parent_id = ?", id).Count(&count).Error
	if err != nil {
		return false, apperror.Internal(err)
	}
	return count > 0, nil
}

func (s *gormFolderStore) Update(ctx context.Context, f *folder.Folder) error {
	if err := s.db.WithContext(ctx).Save(f).Error; err != nil {
		return apperror.Internal(err)
	}
	return nil
}

func (s *gormFolderStore) Delete(ctx context.Context, id uuid.UUID) error {
	result := s.db.WithContext(ctx).Delete(&folder.Folder{}, "id = ?", id)
	if result.Error != nil {
		return apperror.Internal(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperror.NotFound("folder not found", nil)
	}
	return nil
}
