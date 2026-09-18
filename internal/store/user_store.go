package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
	"github.com/laraxpy/photo-bucket-backend/internal/model/user"
	"gorm.io/gorm"
)

type UserStore interface {
	Create(ctx context.Context, user *user.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*user.User, error)
	GetByEmail(ctx context.Context, email string) (*user.User, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	Update(ctx context.Context, user *user.User) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type gormUserStore struct {
	db *gorm.DB
}

func (g *gormUserStore) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	err := g.db.WithContext(ctx).Model(&user.User{}).Where("email=?", email).Count(&count).Error
	if err != nil {
		return false, apperror.Internal(err)
	}
	return count > 0,nil
}

func (g *gormUserStore) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	var user user.User
	err := g.db.WithContext(ctx).First(&user, "email=?", email).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperror.NotFound("This email was not found", err)
	}
	if err != nil{
		return nil, apperror.Internal(err)
	}
	return &user,nil
}

func (g *gormUserStore) Create(ctx context.Context, user *user.User) error {
	if err := g.db.WithContext(ctx).Create(user).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey){
		return apperror.Conflict("This email is already exists", err)
		}
		return apperror.Internal(err)
	}
	return nil
}

func (g *gormUserStore) Delete(ctx context.Context, id uuid.UUID) error {
	result := g.db.WithContext(ctx).Delete(&user.User{}, "id=?", id)
	if result.Error != nil {
		return apperror.Internal(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperror.NotFound("User Not Found", nil)
	}
	return nil
}

func (g *gormUserStore) GetByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	var user user.User
	err := g.db.WithContext(ctx).First(&user, "id= ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperror.NotFound("User not found", err)
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &user, nil
}

func (g *gormUserStore) Update(ctx context.Context, user *user.User) error {
	if err := g.db.WithContext(ctx).Save(user).Error; err != nil {
		return apperror.Internal(err)
	}
	return nil
}

func NewUserStore(db *gorm.DB) UserStore {
	return &gormUserStore{db: db}
}
