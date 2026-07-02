package service_test

import (
	"context"
	"testing"
	"time"

	"tutorgo/models"
	"tutorgo/repository"
	"tutorgo/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockSubRepo struct{ mock.Mock }

func (m *mockSubRepo) GetByTutor(ctx context.Context, tutorID string) (*models.Subscription, error) {
	args := m.Called(ctx, tutorID)
	sub, _ := args.Get(0).(*models.Subscription)
	return sub, args.Error(1)
}
func (m *mockSubRepo) Activate(ctx context.Context, tutorID, plan string, periodEnd time.Time) error {
	args := m.Called(ctx, tutorID, plan, periodEnd)
	return args.Error(0)
}
func (m *mockSubRepo) CreateTrialTx(ctx context.Context, q repository.Querier, tutorID string) error {
	args := m.Called(ctx, q, tutorID)
	return args.Error(0)
}

func TestSubscriptionService_State_NoRowBlocked(t *testing.T) {
	repo := new(mockSubRepo)
	repo.On("GetByTutor", mock.Anything, "t1").Return((*models.Subscription)(nil), nil)
	svc := service.NewSubscriptionService(repo)

	state, err := svc.State(context.Background(), "t1")
	assert.NoError(t, err)
	assert.Equal(t, service.StateBlocked, state)
}

func TestSubscriptionService_GetStatus_IncludesPrices(t *testing.T) {
	repo := new(mockSubRepo)
	repo.On("GetByTutor", mock.Anything, "t1").Return(&models.Subscription{Grandfathered: true}, nil)
	svc := service.NewSubscriptionService(repo)

	st, err := svc.GetStatus(context.Background(), "t1")
	assert.NoError(t, err)
	assert.Equal(t, service.StateActive, st.State)
	assert.Equal(t, service.PriceMonthly, st.Prices.Monthly)
	assert.Equal(t, service.Currency, st.Prices.Currency)
}

func TestSubscriptionService_Confirm_ActivatesMonthly(t *testing.T) {
	repo := new(mockSubRepo)
	repo.On("Activate", mock.Anything, "t1", "monthly", mock.MatchedBy(func(pe time.Time) bool {
		// период ~30 дней вперёд
		return pe.After(time.Now().Add(29*24*time.Hour)) && pe.Before(time.Now().Add(31*24*time.Hour))
	})).Return(nil)
	svc := service.NewSubscriptionService(repo)

	err := svc.Confirm(context.Background(), "t1", "monthly")
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}
