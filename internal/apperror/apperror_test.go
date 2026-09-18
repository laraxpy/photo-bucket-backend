package apperror

import (
	"errors"
	"net/http"
	"testing"
)

func TestAppError_Error(t *testing.T) {
	t.Run("with wrapped error", func(t *testing.T) {
		wrapped := errors.New("boom")
		err := Internal(wrapped)
		if got := err.Error(); got != "internal server error: boom" {
			t.Errorf("Error() = %q, want %q", got, "internal server error: boom")
		}
	})

	t.Run("without wrapped error", func(t *testing.T) {
		err := NotFound("not found", nil)
		if got := err.Error(); got != "not found" {
			t.Errorf("Error() = %q, want %q", got, "not found")
		}
	})
}

func TestAppError_Unwrap(t *testing.T) {
	wrapped := errors.New("root cause")
	err := Internal(wrapped)
	if !errors.Is(err, wrapped) {
		t.Error("expected errors.Is to unwrap to the original error")
	}
}

func TestConstructors_HTTPStatus(t *testing.T) {
	cases := []struct {
		name string
		err  *AppError
		want int
	}{
		{"BadRequest", BadRequest("x", nil), http.StatusBadRequest},
		{"Unauthorized", Unauthorized("x", nil), http.StatusUnauthorized},
		{"Forbidden", Forbidden("x", nil), http.StatusForbidden},
		{"NotFound", NotFound("x", nil), http.StatusNotFound},
		{"Conflict", Conflict("x", nil), http.StatusConflict},
		{"Validation", Validation("x", nil), http.StatusBadRequest},
		{"NoMethod", NoMethod("x", nil), http.StatusMethodNotAllowed},
		{"Internal", Internal(nil), http.StatusInternalServerError},
		{"TooManyRequest", TooManyRequest("x", nil), http.StatusTooManyRequests},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err.HTTPStatus != tc.want {
				t.Errorf("HTTPStatus = %d, want %d", tc.err.HTTPStatus, tc.want)
			}
		})
	}
}

func TestFrom(t *testing.T) {
	t.Run("nil error returns nil", func(t *testing.T) {
		if From(nil) != nil {
			t.Error("From(nil) should return nil")
		}
	})

	t.Run("already an AppError is returned as-is", func(t *testing.T) {
		original := Conflict("dup", nil)
		if got := From(original); got != original {
			t.Errorf("From should return the same *AppError, got %v", got)
		}
	})

	t.Run("plain error is wrapped as Internal", func(t *testing.T) {
		got := From(errors.New("plain"))
		if got.Code != CodeInternal {
			t.Errorf("Code = %v, want %v", got.Code, CodeInternal)
		}
	})
}

func TestRequiredFieldsFrom(t *testing.T) {
	type sample struct {
		Name  string `binding:"required,min=3"`
		Email string `binding:"required,email"`
		Bio   string `binding:"max=200"`
	}

	got := RequiredFieldsFrom(&sample{})
	want := map[string]bool{"Name": true, "Email": true}

	if len(got) != len(want) {
		t.Fatalf("RequiredFieldsFrom returned %v, want fields matching %v", got, want)
	}
	for _, field := range got {
		if !want[field] {
			t.Errorf("unexpected required field %q", field)
		}
	}
}
