package service_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"tutorgo/models"
	"tutorgo/repository"
	"tutorgo/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type fakeProvider struct {
	initURL     string
	initErr     error
	chargeID    string
	chargeErr   error
	statusValue string // ответ CheckStatus (reconciliation)
	statusErr   error
	callback    service.Callback
	callbackErr error
	lastCharge  struct {
		orderID, token string
		amount         int
	}
}

func (f *fakeProvider) InitPayment(_ context.Context, _, _, _ string, _ int) (string, error) {
	return f.initURL, f.initErr
}
func (f *fakeProvider) Charge(_ context.Context, orderID, token string, amount int) (string, error) {
	f.lastCharge.orderID, f.lastCharge.token, f.lastCharge.amount = orderID, token, amount
	return f.chargeID, f.chargeErr
}
func (f *fakeProvider) CheckStatus(_ context.Context, _ string) (string, error) {
	return f.statusValue, f.statusErr
}
func (f *fakeProvider) ParseCallback(_ *http.Request) (service.Callback, error) {
	return f.callback, f.callbackErr
}

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
	svc := service.NewSubscriptionService(repo, &fakeProvider{})

	state, err := svc.State(context.Background(), "t1")
	assert.NoError(t, err)
	assert.Equal(t, service.StateBlocked, state)
}

func TestSubscriptionService_GetStatus_IncludesPrices(t *testing.T) {
	repo := new(mockSubRepo)
	repo.On("GetByTutor", mock.Anything, "t1").Return(&models.Subscription{Grandfathered: true}, nil)
	svc := service.NewSubscriptionService(repo, &fakeProvider{})

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
	svc := service.NewSubscriptionService(repo, &fakeProvider{})

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
	svc := service.NewSubscriptionService(repo, &fakeProvider{})

	err := svc.Confirm(context.Background(), "t1", "yearly")
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestCheckout_InsertsPendingAndReturnsURL(t *testing.T) {
	repo := new(mockSubRepo)
	prov := &fakeProvider{initURL: "https://pay.freedom/redirect"}
	repo.On("InsertPendingPayment", mock.Anything, "t1", mock.AnythingOfType("string"), "monthly", service.PriceMonthly).
		Return(true, nil)
	svc := service.NewSubscriptionService(repo, prov)

	url, err := svc.Checkout(context.Background(), "t1", "monthly")
	assert.NoError(t, err)
	assert.Equal(t, "https://pay.freedom/redirect", url)
	repo.AssertExpectations(t)
}

func TestHandleWebhook_Success_StartsPeriod(t *testing.T) {
	repo := new(mockSubRepo)
	prov := &fakeProvider{callback: service.Callback{
		OrderID: "o1", ProviderPaymentID: "pp1", Status: "success", CardToken: "tok1",
	}}
	repo.On("GetPaymentByOrderID", mock.Anything, "o1").
		Return(&models.SubscriptionPayment{TutorID: "t1", OrderID: "o1", Plan: "monthly", Amount: 10000, Status: "pending"}, nil)
	repo.On("MarkPaymentSuccess", mock.Anything, "o1", "pp1").Return(true, nil)
	repo.On("StartPaidPeriod", mock.Anything, "t1", "monthly", "tok1", mock.AnythingOfType("time.Time")).Return(nil)
	svc := service.NewSubscriptionService(repo, prov)

	err := svc.HandleWebhook(context.Background(), httptest.NewRequest("POST", "/subscription/webhook", nil))
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestHandleWebhook_Dedup_NoOp(t *testing.T) {
	repo := new(mockSubRepo)
	prov := &fakeProvider{callback: service.Callback{OrderID: "o1", ProviderPaymentID: "pp1", Status: "success"}}
	repo.On("GetPaymentByOrderID", mock.Anything, "o1").
		Return(&models.SubscriptionPayment{TutorID: "t1", Plan: "monthly", Status: "success"}, nil)
	repo.On("MarkPaymentSuccess", mock.Anything, "o1", "pp1").Return(false, nil) // уже success
	svc := service.NewSubscriptionService(repo, prov)

	err := svc.HandleWebhook(context.Background(), httptest.NewRequest("POST", "/subscription/webhook", nil))
	assert.NoError(t, err)
	repo.AssertNotCalled(t, "StartPaidPeriod", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestHandleWebhook_BadSignature(t *testing.T) {
	repo := new(mockSubRepo)
	prov := &fakeProvider{callbackErr: errors.New("bad sig")}
	svc := service.NewSubscriptionService(repo, prov)

	err := svc.HandleWebhook(context.Background(), httptest.NewRequest("POST", "/subscription/webhook", nil))
	assert.ErrorIs(t, err, service.ErrBadSignature)
}

func TestRenewDue_ChargesAndExtends(t *testing.T) {
	repo := new(mockSubRepo)
	prov := &fakeProvider{chargeID: "pp-mit"}
	pe := time.Now().Add(-time.Hour)
	repo.On("ListDueAutopay", mock.Anything, mock.AnythingOfType("time.Time")).
		Return([]models.DueSubscription{{TutorID: "t1", Plan: "monthly", CardToken: "tok1", PeriodEnd: pe}}, nil)
	repo.On("InsertPendingPayment", mock.Anything, "t1", mock.AnythingOfType("string"), "monthly", service.PriceMonthly).Return(true, nil)
	repo.On("MarkPaymentSuccess", mock.Anything, mock.AnythingOfType("string"), "pp-mit").Return(true, nil)
	repo.On("RenewPeriod", mock.Anything, "t1", "monthly", mock.AnythingOfType("time.Time")).Return(nil)
	svc := service.NewSubscriptionService(repo, prov)

	n, err := svc.RenewDue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(1), n)
	repo.AssertExpectations(t)
}

func TestRenewDue_AppliesPendingPlan(t *testing.T) {
	repo := new(mockSubRepo)
	prov := &fakeProvider{chargeID: "pp-mit"}
	yearly := "yearly"
	repo.On("ListDueAutopay", mock.Anything, mock.Anything).
		Return([]models.DueSubscription{{TutorID: "t1", Plan: "monthly", PendingPlan: &yearly, CardToken: "tok1", PeriodEnd: time.Now()}}, nil)
	// эффективный план — yearly: claim и charge на yearly-сумму, продление на yearly
	repo.On("InsertPendingPayment", mock.Anything, "t1", mock.AnythingOfType("string"), "yearly", service.PriceYearly).Return(true, nil)
	repo.On("MarkPaymentSuccess", mock.Anything, mock.AnythingOfType("string"), "pp-mit").Return(true, nil)
	repo.On("RenewPeriod", mock.Anything, "t1", "yearly", mock.AnythingOfType("time.Time")).Return(nil)
	svc := service.NewSubscriptionService(repo, prov)

	n, err := svc.RenewDue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(1), n)
	repo.AssertExpectations(t)
}

func TestRenewDue_ChargeFails_MarksFailedNoExtend(t *testing.T) {
	repo := new(mockSubRepo)
	prov := &fakeProvider{chargeErr: errors.New("declined")}
	repo.On("ListDueAutopay", mock.Anything, mock.Anything).
		Return([]models.DueSubscription{{TutorID: "t1", Plan: "monthly", CardToken: "tok1", PeriodEnd: time.Now()}}, nil)
	repo.On("InsertPendingPayment", mock.Anything, "t1", mock.AnythingOfType("string"), "monthly", service.PriceMonthly).Return(true, nil)
	repo.On("MarkPaymentFailed", mock.Anything, mock.AnythingOfType("string")).Return(nil)
	svc := service.NewSubscriptionService(repo, prov)

	n, err := svc.RenewDue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(0), n)
	repo.AssertNotCalled(t, "RenewPeriod", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestRenewDue_ChargeErrButReconcileSuccess_Extends(t *testing.T) {
	repo := new(mockSubRepo)
	// Charge упал (таймаут), но CheckStatus говорит success → трактуем как оплату.
	prov := &fakeProvider{chargeErr: errors.New("timeout"), statusValue: "success"}
	repo.On("ListDueAutopay", mock.Anything, mock.Anything).
		Return([]models.DueSubscription{{TutorID: "t1", Plan: "monthly", CardToken: "tok1", PeriodEnd: time.Now()}}, nil)
	repo.On("InsertPendingPayment", mock.Anything, "t1", mock.AnythingOfType("string"), "monthly", service.PriceMonthly).Return(true, nil)
	repo.On("MarkPaymentSuccess", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(true, nil)
	repo.On("RenewPeriod", mock.Anything, "t1", "monthly", mock.AnythingOfType("time.Time")).Return(nil)
	svc := service.NewSubscriptionService(repo, prov)

	n, err := svc.RenewDue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(1), n)
	repo.AssertNotCalled(t, "MarkPaymentFailed", mock.Anything, mock.Anything)
}

func TestRenewDue_ChargeErrReconcileFailed_MarksFailed(t *testing.T) {
	repo := new(mockSubRepo)
	prov := &fakeProvider{chargeErr: errors.New("declined"), statusValue: "failed"}
	repo.On("ListDueAutopay", mock.Anything, mock.Anything).
		Return([]models.DueSubscription{{TutorID: "t1", Plan: "monthly", CardToken: "tok1", PeriodEnd: time.Now()}}, nil)
	repo.On("InsertPendingPayment", mock.Anything, "t1", mock.AnythingOfType("string"), "monthly", service.PriceMonthly).Return(true, nil)
	repo.On("MarkPaymentFailed", mock.Anything, mock.AnythingOfType("string")).Return(nil)
	svc := service.NewSubscriptionService(repo, prov)

	n, err := svc.RenewDue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(0), n)
	repo.AssertNotCalled(t, "RenewPeriod", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestRenewDue_NotClaimed_SkipsCharge(t *testing.T) {
	repo := new(mockSubRepo)
	prov := &fakeProvider{}
	repo.On("ListDueAutopay", mock.Anything, mock.Anything).
		Return([]models.DueSubscription{{TutorID: "t1", Plan: "monthly", CardToken: "tok1", PeriodEnd: time.Now()}}, nil)
	repo.On("InsertPendingPayment", mock.Anything, "t1", mock.AnythingOfType("string"), "monthly", service.PriceMonthly).Return(false, nil)
	svc := service.NewSubscriptionService(repo, prov)

	n, err := svc.RenewDue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(0), n)
	assert.Equal(t, "", prov.lastCharge.token) // Charge не вызывался
}

func TestCancel(t *testing.T) {
	repo := new(mockSubRepo)
	repo.On("Cancel", mock.Anything, "t1").Return(nil)
	svc := service.NewSubscriptionService(repo, &fakeProvider{})
	assert.NoError(t, svc.Cancel(context.Background(), "t1"))
	repo.AssertExpectations(t)
}

func TestChangePlan(t *testing.T) {
	repo := new(mockSubRepo)
	repo.On("SetPendingPlan", mock.Anything, "t1", "yearly").Return(nil)
	svc := service.NewSubscriptionService(repo, &fakeProvider{})
	assert.NoError(t, svc.ChangePlan(context.Background(), "t1", "yearly"))
	repo.AssertExpectations(t)
}
