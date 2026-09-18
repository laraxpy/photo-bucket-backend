package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret"

func init() {
	gin.SetMode(gin.TestMode)
}

func runAuth(t *testing.T, authHeader string) (*httptest.ResponseRecorder, *gin.Context) {
	t.Helper()
	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)
	// ErrorHandler must run in the chain: AuthRequired only records
	// c.Error()+c.Abort(), ErrorHandler is what writes the actual HTTP status.
	r.Use(ErrorHandler())
	r.Use(AuthRequired(testSecret))
	r.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	c.Request = req
	r.ServeHTTP(w, req)
	return w, c
}

func signToken(t *testing.T, method jwt.SigningMethod, claims jwt.MapClaims, secret []byte) string {
	t.Helper()
	token := jwt.NewWithClaims(method, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return signed
}

func TestAuthRequired_MissingHeader(t *testing.T) {
	w, _ := runAuth(t, "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthRequired_MissingBearerPrefix(t *testing.T) {
	w, _ := runAuth(t, "sometoken")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthRequired_GarbageToken(t *testing.T) {
	w, _ := runAuth(t, "Bearer not-a-real-jwt")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthRequired_ValidToken(t *testing.T) {
	claims := jwt.MapClaims{
		"sub": "11111111-1111-1111-1111-111111111111",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	token := signToken(t, jwt.SigningMethodHS256, claims, []byte(testSecret))

	w, _ := runAuth(t, "Bearer "+token)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAuthRequired_ExpiredToken(t *testing.T) {
	claims := jwt.MapClaims{
		"sub": "11111111-1111-1111-1111-111111111111",
		"exp": time.Now().Add(-time.Hour).Unix(),
	}
	token := signToken(t, jwt.SigningMethodHS256, claims, []byte(testSecret))

	w, _ := runAuth(t, "Bearer "+token)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthRequired_WrongSecret(t *testing.T) {
	claims := jwt.MapClaims{
		"sub": "11111111-1111-1111-1111-111111111111",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	token := signToken(t, jwt.SigningMethodHS256, claims, []byte("a-different-secret"))

	w, _ := runAuth(t, "Bearer "+token)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

// TestAuthRequired_RejectsAlgNone guards against the classic JWT "alg confusion"
// attack: a token that declares alg=none and carries no signature must never
// be accepted, regardless of what jwtSecret is configured.
func TestAuthRequired_RejectsAlgNone(t *testing.T) {
	claims := jwt.MapClaims{
		"sub": "11111111-1111-1111-1111-111111111111",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	unsigned, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("failed to build alg=none token: %v", err)
	}

	w, _ := runAuth(t, "Bearer "+unsigned)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("alg=none token must be rejected, got status %d", w.Code)
	}
}
