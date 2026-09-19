package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
	"github.com/laraxpy/photo-bucket-backend/internal/model/token"
	"github.com/laraxpy/photo-bucket-backend/internal/model/user"
	"github.com/laraxpy/photo-bucket-backend/internal/store"
	"golang.org/x/crypto/bcrypt"
)

const (
	accessTokenTTL  = 24 * time.Hour
	refreshTokenTTL = 30 * 24 * time.Hour
)

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

type UserService interface {
	Register(ctx context.Context, name, email, password string) (*user.User, error)
	Login(ctx context.Context, email, password string) (TokenPair, error)
	Refresh(ctx context.Context, refreshToken string) (TokenPair, error)
}

type userService struct {
	userStore         store.UserStore
	refreshTokenStore store.RefreshTokenStore
	jwtSecret         []byte
}

func NewUserService(userStore store.UserStore, refreshTokenStore store.RefreshTokenStore, jwtSecret string) UserService {
	return &userService{userStore: userStore, refreshTokenStore: refreshTokenStore, jwtSecret: []byte(jwtSecret)}
}

func (s *userService) Login(ctx context.Context, email string, password string) (TokenPair, error) {
	user, err := s.userStore.GetByEmail(ctx, email)
	if err != nil {
		return TokenPair{}, apperror.Unauthorized("invalid credentials", nil)
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return TokenPair{}, apperror.Unauthorized("invalid credentials", nil)
	}

	return s.issueTokenPair(ctx, user.ID)
}

func (s *userService) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	hash := hashToken(refreshToken)

	stored, err := s.refreshTokenStore.GetByTokenHash(ctx, hash)
	if err != nil {
		return TokenPair{}, err
	}
	if stored.RevokedAt != nil || time.Now().After(stored.ExpiresAt) {
		return TokenPair{}, apperror.Unauthorized("invalid refresh token", nil)
	}

	if err := s.refreshTokenStore.Revoke(ctx, stored.ID.String()); err != nil {
		return TokenPair{}, err
	}

	return s.issueTokenPair(ctx, stored.UserID)
}

func (s *userService) issueTokenPair(ctx context.Context, userID uuid.UUID) (TokenPair, error) {
	accessToken, err := s.signAccessToken(userID)
	if err != nil {
		return TokenPair{}, err
	}

	refreshToken, err := generateRefreshToken()
	if err != nil {
		return TokenPair{}, apperror.Internal(err)
	}

	record := &token.RefreshToken{
		UserID:    userID,
		TokenHash: hashToken(refreshToken),
		ExpiresAt: time.Now().Add(refreshTokenTTL),
	}
	if err := s.refreshTokenStore.Create(ctx, record); err != nil {
		return TokenPair{}, err
	}

	return TokenPair{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

func (s *userService) signAccessToken(userID uuid.UUID) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID.String(),
		"exp": time.Now().Add(accessTokenTTL).Unix(),
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

func generateRefreshToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
