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
func (m *mockSubRepo) InsertPendingPayment(ctx context.Context, tutorID, orderID, plan string, amount int) (bool, error) {
	args := m.Called(ctx, tutorID, orderID, plan, amount)
	return args.Bool(0), args.Error(1)
}
func (m *mockSubRepo) MarkPaymentSuccess(ctx context.Context, orderID, ppid string) (bool, error) {
	args := m.Called(ctx, orderID, ppid)
	return args.Bool(0), args.Error(1)
}
func (m *mockSubRepo) MarkPaymentFailed(ctx context.Context, orderID string) error {
	return m.Called(ctx, orderID).Error(0)
}
func (m *mockSubRepo) GetPaymentByOrderID(ctx context.Context, orderID string) (*models.SubscriptionPayment, error) {
	args := m.Called(ctx, orderID)
	p, _ := args.Get(0).(*models.SubscriptionPayment)
	return p, args.Error(1)
}
func (m *mockSubRepo) StartPaidPeriod(ctx context.Context, tutorID, plan, token string, pe time.Time) error {
	return m.Called(ctx, tutorID, plan, token, pe).Error(0)
}
func (m *mockSubRepo) RenewPeriod(ctx context.Context, tutorID, plan string, pe time.Time) error {
	return m.Called(ctx, tutorID, plan, pe).Error(0)
}
func (m *mockSubRepo) ListDueAutopay(ctx context.Context, now time.Time) ([]models.DueSubscription, error) {
	args := m.Called(ctx, now)
	d, _ := args.Get(0).([]models.DueSubscription)
	return d, args.Error(1)
}
func (m *mockSubRepo) Cancel(ctx context.Context, tutorID string) error {
	return m.Called(ctx, tutorID).Error(0)
}
func (m *mockSubRepo) SetPendingPlan(ctx context.Context, tutorID, plan string) error {
	return m.Called(ctx, tutorID, plan).Error(0)
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

func TestSubscriptionService_Confirm_ActivatesYearly(t *testing.T) {
	repo := new(mockSubRepo)
	repo.On("Activate", mock.Anything, "t1", "yearly", mock.MatchedBy(func(pe time.Time) bool {
		// период ~365 дней вперёд
		return pe.After(time.Now().Add(364*24*time.Hour)) && pe.Before(time.Now().Add(366*24*time.Hour))
	})).Return(nil)
	svc := service.NewSubscriptionService(repo)

	err := svc.Confirm(context.Background(), "t1", "yearly")
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}
