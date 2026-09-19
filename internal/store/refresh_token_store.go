package store

import (
	"context"
	"errors"
	"time"

	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
	"github.com/laraxpy/photo-bucket-backend/internal/model/token"
	"gorm.io/gorm"
)

type RefreshTokenStore interface {
	Create(ctx context.Context, t *token.RefreshToken) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*token.RefreshToken, error)
	Revoke(ctx context.Context, id string) error
}

type gormRefreshTokenStore struct {
	db *gorm.DB
}

func NewRefreshTokenStore(db *gorm.DB) RefreshTokenStore {
	return &gormRefreshTokenStore{db: db}
}

func (s *gormRefreshTokenStore) Create(ctx context.Context, t *token.RefreshToken) error {
	if err := s.db.WithContext(ctx).Create(t).Error; err != nil {
		return apperror.Internal(err)
	}
	return nil
}

func (s *gormRefreshTokenStore) GetByTokenHash(ctx context.Context, tokenHash string) (*token.RefreshToken, error) {
	var t token.RefreshToken
	err := s.db.WithContext(ctx).First(&t, "token_hash = ?", tokenHash).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperror.Unauthorized("invalid refresh token", nil)
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &t, nil
}

func (s *gormRefreshTokenStore) Revoke(ctx context.Context, id string) error {
	err := s.db.WithContext(ctx).Model(&token.RefreshToken{}).
		Where("id = ?", id).
		Update("revoked_at", time.Now()).Error
	if err != nil {
		return apperror.Internal(err)
	}
	return nil
}
