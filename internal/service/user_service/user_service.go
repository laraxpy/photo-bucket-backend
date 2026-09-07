package service

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
	"github.com/laraxpy/photo-bucket-backend/internal/model/user"
	"github.com/laraxpy/photo-bucket-backend/internal/store"
	"golang.org/x/crypto/bcrypt"
)

type UserService interface {
	Register(ctx context.Context, name, email, password string) (*user.User, error)
	Login(ctx context.Context, email, password string) (string, error)
}

type userService struct {
	userStore store.UserStore
	jwtSecret []byte
}

func NewUserService(userStore store.UserStore, jwtSecret string) UserService {
	return &userService{userStore: userStore, jwtSecret: []byte(jwtSecret)}
}
func (s *userService) Login(ctx context.Context, email string, password string) (string, error) {
	user, err := s.userStore.GetByEmail(ctx, email)
	if err != nil {
		return "", apperror.Unauthorized("invalid credentials", nil)
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return "", apperror.Unauthorized("invalid credentials", nil)
	}
	claims := jwt.MapClaims{
		"sub": user.ID.String(),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", apperror.Internal(err)
	}
	return signedToken, nil
}

func (s *userService) Register(ctx context.Context, name, email, password string) (*user.User, error) {
	exists, err := s.userStore.ExistsByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperror.Conflict("this email already exists", nil)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	user := &user.User{
		Name:         name,
		Email:        email,
		PasswordHash: string(hash),
	}

	if err := s.userStore.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}
