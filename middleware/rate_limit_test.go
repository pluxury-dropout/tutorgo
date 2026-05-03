package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"tutorgo/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

func init() { gin.SetMode(gin.TestMode) }

func newLimitedRouter(r rate.Limit, b int) *gin.Engine {
	router := gin.New()
	router.Use(middleware.RateLimit(r, b))
	router.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })
	return router
}

func TestRateLimit_AllowsWithinBurst(t *testing.T) {
	router := newLimitedRouter(rate.Every(time.Minute), 3)
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "request %d should pass", i+1)
	}
}

func TestRateLimit_BlocksExceedingBurst(t *testing.T) {
	router := newLimitedRouter(rate.Every(time.Minute), 2)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "5.6.7.8:1234"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "5.6.7.8:1234"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestRateLimit_DifferentIPsAreIndependent(t *testing.T) {
	router := newLimitedRouter(rate.Every(time.Minute), 1)
	for _, ip := range []string{"10.0.0.1:1", "10.0.0.2:1", "10.0.0.3:1"} {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = ip
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "IP %s should have its own limiter", ip)
	}
}
