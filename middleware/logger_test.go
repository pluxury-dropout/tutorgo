package middleware_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"tutorgo/middleware"
)

func TestLogger_LogsRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	capture := &captureHandler{}
	log := slog.New(capture)

	r := gin.New()
	r.Use(middleware.Logger(log))
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, capture.called, "Logger should have logged the request")
	assert.Equal(t, "http", capture.msg)
}

type captureHandler struct {
	called bool
	msg    string
	attrs  []slog.Attr
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.called = true
	h.msg = r.Message
	r.Attrs(func(a slog.Attr) bool {
		h.attrs = append(h.attrs, a)
		return true
	})
	return nil
}
func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(name string) slog.Handler        { return h }
