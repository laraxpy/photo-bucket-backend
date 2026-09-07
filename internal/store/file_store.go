package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
	"github.com/laraxpy/photo-bucket-backend/internal/model/file"
	"gorm.io/gorm"
)

type FileStore interface {
	Create(ctx context.Context, f *file.File) error
	GetByID(ctx context.Context, id uuid.UUID) (*file.File, error)
	GetByObjectKey(ctx context.Context, objectKey string) (*file.File, error)
	ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]file.File, error)
	Update(ctx context.Context, f *file.File) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status file.FileStatus) error
	Delete(ctx context.Context, id uuid.UUID) error
	HardDelete(ctx context.Context, id uuid.UUID) error
	CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error)
}

type gormFileStore struct {
	db *gorm.DB
}

func NewFileStore(db *gorm.DB) FileStore {
	return &gormFileStore{db: db}
}

func (s *gormFileStore) Create(ctx context.Context, f *file.File) error {
	if err := s.db.WithContext(ctx).Create(f).Error; err != nil {
		return apperror.Internal(err)
	}
	return nil
}

func (s *gormFileStore) GetByID(ctx context.Context, id uuid.UUID) (*file.File, error) {
	var f file.File
	err := s.db.WithContext(ctx).First(&f, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperror.NotFound("file not found", err)
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &f, nil
}

func (s *gormFileStore) GetByObjectKey(ctx context.Context, objectKey string) (*file.File, error) {
	var f file.File
	err := s.db.WithContext(ctx).First(&f, "object_key = ?", objectKey).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperror.NotFound("file not found", err)
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &f, nil
}

func (s *gormFileStore) ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]file.File, error) {
	var files []file.File
	err := s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&files).Error
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return files, nil
}

func (s *gormFileStore) Update(ctx context.Context, f *file.File) error {
	if err := s.db.WithContext(ctx).Save(f).Error; err != nil {
		return apperror.Internal(err)
	}
	return nil
}

func (s *gormFileStore) UpdateStatus(ctx context.Context, id uuid.UUID, status file.FileStatus) error {
	result := s.db.WithContext(ctx).
		Model(&file.File{}).
		Where("id = ?", id).
		Update("status", status)

	if result.Error != nil {
		return apperror.Internal(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperror.NotFound("file not found", nil)
	}
	return nil
}

func (s *gormFileStore) Delete(ctx context.Context, id uuid.UUID) error {
	result := s.db.WithContext(ctx).Delete(&file.File{}, "id = ?", id)
	if result.Error != nil {
		return apperror.Internal(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperror.NotFound("file not found", nil)
	}
	return nil
}

func (s *gormFileStore) HardDelete(ctx context.Context, id uuid.UUID) error {
	result := s.db.WithContext(ctx).Unscoped().Delete(&file.File{}, "id = ?", id)
	if result.Error != nil {
		return apperror.Internal(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperror.NotFound("file not found", nil)
	}
	return nil
}

func (s *gormFileStore) CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&file.File{}).
		Where("user_id = ?", userID).
		Count(&count).Error
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return count, nil
}
