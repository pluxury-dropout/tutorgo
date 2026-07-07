package handlers_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"tutorgo/handlers"
	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockSubscriptionService struct{ mock.Mock }

func (m *mockSubscriptionService) GetStatus(ctx context.Context, tutorID string) (models.SubscriptionStatus, error) {
	args := m.Called(ctx, tutorID)
	st, _ := args.Get(0).(models.SubscriptionStatus)
	return st, args.Error(1)
}

func (m *mockSubscriptionService) State(ctx context.Context, tutorID string) (string, error) {
	args := m.Called(ctx, tutorID)
	return args.String(0), args.Error(1)
}

func (m *mockSubscriptionService) Checkout(ctx context.Context, tutorID, plan string) (string, error) {
	args := m.Called(ctx, tutorID, plan)
	return args.String(0), args.Error(1)
}

func (m *mockSubscriptionService) Confirm(ctx context.Context, tutorID, plan string) error {
	return m.Called(ctx, tutorID, plan).Error(0)
}

func (m *mockSubscriptionService) HandleWebhook(ctx context.Context, r *http.Request) error {
	return m.Called(ctx, r).Error(0)
}

func (m *mockSubscriptionService) RenewDue(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockSubscriptionService) Cancel(ctx context.Context, tutorID string) error {
	return m.Called(ctx, tutorID).Error(0)
}

func (m *mockSubscriptionService) ChangePlan(ctx context.Context, tutorID, plan string) error {
	return m.Called(ctx, tutorID, plan).Error(0)
}

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestWebhook_BadSignature_401(t *testing.T) {
	svc := new(mockSubscriptionService)
	svc.On("HandleWebhook", mock.Anything, mock.Anything).Return(service.ErrBadSignature)
	h := handlers.NewSubscriptionHandler(svc, testLogger())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/subscription/webhook", nil)
	h.Webhook(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestWebhook_OK_200(t *testing.T) {
	svc := new(mockSubscriptionService)
	svc.On("HandleWebhook", mock.Anything, mock.Anything).Return(nil)
	h := handlers.NewSubscriptionHandler(svc, testLogger())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/subscription/webhook", nil)
	h.Webhook(c)
	assert.Equal(t, http.StatusOK, w.Code)
}
