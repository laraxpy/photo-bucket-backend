package service

import (
	"context"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
	"github.com/laraxpy/photo-bucket-backend/internal/model/user"
)

type fakeUserStore struct {
	byID    map[uuid.UUID]user.User
	byEmail map[string]uuid.UUID
}

func newFakeUserStore() *fakeUserStore {
	return &fakeUserStore{byID: make(map[uuid.UUID]user.User), byEmail: make(map[string]uuid.UUID)}
}

func (s *fakeUserStore) Create(ctx context.Context, u *user.User) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	if _, exists := s.byEmail[u.Email]; exists {
		return apperror.Conflict("this email already exists", nil)
	}
	s.byID[u.ID] = *u
	s.byEmail[u.Email] = u.ID
	return nil
}

func (s *fakeUserStore) GetByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	u, ok := s.byID[id]
	if !ok {
		return nil, apperror.NotFound("user not found", nil)
	}
	return &u, nil
}

func (s *fakeUserStore) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	id, ok := s.byEmail[email]
	if !ok {
		return nil, apperror.NotFound("This email was not found", nil)
	}
	u := s.byID[id]
	return &u, nil
}

func (s *fakeUserStore) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	_, ok := s.byEmail[email]
	return ok, nil
}

func (s *fakeUserStore) Update(ctx context.Context, u *user.User) error {
	if _, ok := s.byID[u.ID]; !ok {
		return apperror.NotFound("user not found", nil)
	}
	s.byID[u.ID] = *u
	return nil
}

func (s *fakeUserStore) Delete(ctx context.Context, id uuid.UUID) error {
	u, ok := s.byID[id]
	if !ok {
		return apperror.NotFound("user not found", nil)
	}
	delete(s.byID, id)
	delete(s.byEmail, u.Email)
	return nil
}

func TestUserService_Register(t *testing.T) {
	ctx := context.Background()

	t.Run("registers a new user with a hashed password", func(t *testing.T) {
		store := newFakeUserStore()
		svc := NewUserService(store, newFakeRefreshTokenStore(), "secret")

		u, err := svc.Register(ctx, "Ana", "ana@example.com", "password123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if u.PasswordHash == "" || u.PasswordHash == "password123" {
			t.Error("expected the password to be hashed, not stored in plain text")
		}
	})

	t.Run("rejects a duplicate email", func(t *testing.T) {
		store := newFakeUserStore()
		svc := NewUserService(store, newFakeRefreshTokenStore(), "secret")

		if _, err := svc.Register(ctx, "Ana", "ana@example.com", "password123"); err != nil {
			t.Fatalf("unexpected error on first register: %v", err)
		}
		_, err := svc.Register(ctx, "Ana 2", "ana@example.com", "otherpassword")
		if err == nil {
			t.Fatal("expected a conflict error for a duplicate email")
		}
		if appErr := apperror.From(err); appErr.Code != apperror.CodeConflict {
			t.Errorf("Code = %v, want %v", appErr.Code, apperror.CodeConflict)
		}
	})
}

func TestUserService_Login(t *testing.T) {
	ctx := context.Background()
	const secret = "top-secret"

	t.Run("returns a valid JWT and refresh token on correct credentials", func(t *testing.T) {
		store := newFakeUserStore()
		svc := NewUserService(store, newFakeRefreshTokenStore(), secret)
		registered, err := svc.Register(ctx, "Ana", "ana@example.com", "password123")
		if err != nil {
			t.Fatalf("unexpected error registering: %v", err)
		}

		tokens, err := svc.Login(ctx, "ana@example.com", "password123")
		if err != nil {
			t.Fatalf("unexpected error logging in: %v", err)
		}
		if tokens.RefreshToken == "" {
			t.Fatal("expected a non-empty refresh token")
		}

		parsed, err := jwt.Parse(tokens.AccessToken, func(t *jwt.Token) (any, error) {
			return []byte(secret), nil
		})
		if err != nil || !parsed.Valid {
			t.Fatalf("expected a valid JWT, got parse error: %v", err)
		}
		claims := parsed.Claims.(jwt.MapClaims)
		if claims["sub"] != registered.ID.String() {
			t.Errorf("sub claim = %v, want %v", claims["sub"], registered.ID.String())
		}
	})

	t.Run("rejects a wrong password", func(t *testing.T) {
		store := newFakeUserStore()
		svc := NewUserService(store, newFakeRefreshTokenStore(), secret)
		if _, err := svc.Register(ctx, "Ana", "ana@example.com", "password123"); err != nil {
			t.Fatalf("unexpected error registering: %v", err)
		}

		_, err := svc.Login(ctx, "ana@example.com", "wrongpassword")
		if err == nil {
			t.Fatal("expected an error for a wrong password")
		}
		if appErr := apperror.From(err); appErr.Code != apperror.CodeUnauthorized {
			t.Errorf("Code = %v, want %v", appErr.Code, apperror.CodeUnauthorized)
		}
	})

	t.Run("rejects an unknown email without revealing it doesn't exist", func(t *testing.T) {
		store := newFakeUserStore()
		svc := NewUserService(store, newFakeRefreshTokenStore(), secret)

		_, err := svc.Login(ctx, "ghost@example.com", "password123")
		if appErr := apperror.From(err); appErr.Code != apperror.CodeUnauthorized {
			t.Errorf("Code = %v, want %v", appErr.Code, apperror.CodeUnauthorized)
		}
	})
}

func TestUserService_Refresh(t *testing.T) {
	ctx := context.Background()
	const secret = "top-secret"

	t.Run("exchanges a valid refresh token for a new token pair", func(t *testing.T) {
		store := newFakeUserStore()
		svc := NewUserService(store, newFakeRefreshTokenStore(), secret)
		if _, err := svc.Register(ctx, "Ana", "ana@example.com", "password123"); err != nil {
			t.Fatalf("unexpected error registering: %v", err)
		}
		original, err := svc.Login(ctx, "ana@example.com", "password123")
		if err != nil {
			t.Fatalf("unexpected error logging in: %v", err)
		}

		refreshed, err := svc.Refresh(ctx, original.RefreshToken)
		if err != nil {
			t.Fatalf("unexpected error refreshing: %v", err)
		}
		if refreshed.AccessToken == "" || refreshed.RefreshToken == "" {
			t.Fatal("expected a new access token and refresh token")
		}
		if refreshed.RefreshToken == original.RefreshToken {
			t.Error("expected the refresh token to rotate on use")
		}
	})

	t.Run("rejects an already-used (rotated) refresh token", func(t *testing.T) {
		store := newFakeUserStore()
		svc := NewUserService(store, newFakeRefreshTokenStore(), secret)
		if _, err := svc.Register(ctx, "Ana", "ana@example.com", "password123"); err != nil {
			t.Fatalf("unexpected error registering: %v", err)
		}
		original, err := svc.Login(ctx, "ana@example.com", "password123")
		if err != nil {
			t.Fatalf("unexpected error logging in: %v", err)
		}

		if _, err := svc.Refresh(ctx, original.RefreshToken); err != nil {
			t.Fatalf("unexpected error on first refresh: %v", err)
		}

		_, err = svc.Refresh(ctx, original.RefreshToken)
		if err == nil {
			t.Fatal("expected an error reusing a rotated refresh token")
		}
		if appErr := apperror.From(err); appErr.Code != apperror.CodeUnauthorized {
			t.Errorf("Code = %v, want %v", appErr.Code, apperror.CodeUnauthorized)
		}
	})

	t.Run("rejects an unknown refresh token", func(t *testing.T) {
		store := newFakeUserStore()
		svc := NewUserService(store, newFakeRefreshTokenStore(), secret)

		_, err := svc.Refresh(ctx, "not-a-real-token")
		if appErr := apperror.From(err); appErr.Code != apperror.CodeUnauthorized {
			t.Errorf("Code = %v, want %v", appErr.Code, apperror.CodeUnauthorized)
		}
	})
}
