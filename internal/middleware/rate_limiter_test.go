package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func runRateLimited(t *testing.T, rateFormatted string, requests int) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.Use(ErrorHandler())
	r.Use(RateLimiter(rateFormatted))
	r.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	for i := 0; i < requests-1; i++ {
		r.ServeHTTP(httptest.NewRecorder(), req)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	w := runRateLimited(t, "5-S", 5)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	w := runRateLimited(t, "5-S", 6)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimiter_FallsBackToDefaultWhenEmpty(t *testing.T) {
	w := runRateLimited(t, "", 1)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}
