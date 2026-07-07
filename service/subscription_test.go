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
func (m *mockSubRepo) MarkPaymentFailed(ctx context.Context, orderID string) error {
	return m.Called(ctx, orderID).Error(0)
}
func (m *mockSubRepo) GetPaymentByOrderID(ctx context.Context, orderID string) (*models.SubscriptionPayment, error) {
	args := m.Called(ctx, orderID)
	p, _ := args.Get(0).(*models.SubscriptionPayment)
	return p, args.Error(1)
}
func (m *mockSubRepo) MarkSuccessAndStartPeriod(ctx context.Context, orderID, ppid, tutorID, plan, token string, pe time.Time) (bool, error) {
	args := m.Called(ctx, orderID, ppid, tutorID, plan, token, pe)
	return args.Bool(0), args.Error(1)
}
func (m *mockSubRepo) MarkSuccessAndRenew(ctx context.Context, orderID, ppid, tutorID, plan string, pe time.Time) (bool, error) {
	args := m.Called(ctx, orderID, ppid, tutorID, plan, pe)
	return args.Bool(0), args.Error(1)
}
func (m *mockSubRepo) ListPeriodPayments(ctx context.Context, prefix string) ([]models.SubscriptionPayment, error) {
	args := m.Called(ctx, prefix)
	p, _ := args.Get(0).([]models.SubscriptionPayment)
	return p, args.Error(1)
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
	repo.On("MarkSuccessAndStartPeriod", mock.Anything, "o1", "pp1", "t1", "monthly", "tok1", mock.AnythingOfType("time.Time")).Return(true, nil)
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
	repo.On("MarkSuccessAndStartPeriod", mock.Anything, "o1", "pp1", "t1", "monthly", "", mock.AnythingOfType("time.Time")).Return(false, nil) // уже success
	svc := service.NewSubscriptionService(repo, prov)

	err := svc.HandleWebhook(context.Background(), httptest.NewRequest("POST", "/subscription/webhook", nil))
	assert.NoError(t, err)
	repo.AssertExpectations(t)
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
	repo.On("ListPeriodPayments", mock.Anything, mock.AnythingOfType("string")).Return(nil, nil)
	repo.On("InsertPendingPayment", mock.Anything, "t1", mock.AnythingOfType("string"), "monthly", service.PriceMonthly).Return(true, nil)
	repo.On("MarkSuccessAndRenew", mock.Anything, mock.AnythingOfType("string"), "pp-mit", "t1", "monthly", mock.AnythingOfType("time.Time")).Return(true, nil)
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
	repo.On("ListPeriodPayments", mock.Anything, mock.AnythingOfType("string")).Return(nil, nil)
	// эффективный план — yearly: claim и charge на yearly-сумму, продление на yearly
	repo.On("InsertPendingPayment", mock.Anything, "t1", mock.AnythingOfType("string"), "yearly", service.PriceYearly).Return(true, nil)
	repo.On("MarkSuccessAndRenew", mock.Anything, mock.AnythingOfType("string"), "pp-mit", "t1", "yearly", mock.AnythingOfType("time.Time")).Return(true, nil)
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
	repo.On("ListPeriodPayments", mock.Anything, mock.AnythingOfType("string")).Return(nil, nil)
	repo.On("InsertPendingPayment", mock.Anything, "t1", mock.AnythingOfType("string"), "monthly", service.PriceMonthly).Return(true, nil)
	repo.On("MarkPaymentFailed", mock.Anything, mock.AnythingOfType("string")).Return(nil)
	svc := service.NewSubscriptionService(repo, prov)

	n, err := svc.RenewDue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(0), n)
	repo.AssertNotCalled(t, "MarkSuccessAndRenew", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestRenewDue_ChargeErrButReconcileSuccess_Extends(t *testing.T) {
	repo := new(mockSubRepo)
	// Charge упал (таймаут), но CheckStatus говорит success → трактуем как оплату.
	prov := &fakeProvider{chargeErr: errors.New("timeout"), statusValue: "success"}
	repo.On("ListDueAutopay", mock.Anything, mock.Anything).
		Return([]models.DueSubscription{{TutorID: "t1", Plan: "monthly", CardToken: "tok1", PeriodEnd: time.Now()}}, nil)
	repo.On("ListPeriodPayments", mock.Anything, mock.AnythingOfType("string")).Return(nil, nil)
	repo.On("InsertPendingPayment", mock.Anything, "t1", mock.AnythingOfType("string"), "monthly", service.PriceMonthly).Return(true, nil)
	repo.On("MarkSuccessAndRenew", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), "t1", "monthly", mock.AnythingOfType("time.Time")).Return(true, nil)
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
	repo.On("ListPeriodPayments", mock.Anything, mock.AnythingOfType("string")).Return(nil, nil)
	repo.On("InsertPendingPayment", mock.Anything, "t1", mock.AnythingOfType("string"), "monthly", service.PriceMonthly).Return(true, nil)
	repo.On("MarkPaymentFailed", mock.Anything, mock.AnythingOfType("string")).Return(nil)
	svc := service.NewSubscriptionService(repo, prov)

	n, err := svc.RenewDue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(0), n)
	repo.AssertNotCalled(t, "MarkSuccessAndRenew", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestRenewDue_NotClaimed_SkipsCharge(t *testing.T) {
	repo := new(mockSubRepo)
	prov := &fakeProvider{}
	repo.On("ListDueAutopay", mock.Anything, mock.Anything).
		Return([]models.DueSubscription{{TutorID: "t1", Plan: "monthly", CardToken: "tok1", PeriodEnd: time.Now()}}, nil)
	repo.On("ListPeriodPayments", mock.Anything, mock.AnythingOfType("string")).Return(nil, nil)
	repo.On("InsertPendingPayment", mock.Anything, "t1", mock.AnythingOfType("string"), "monthly", service.PriceMonthly).Return(false, nil)
	svc := service.NewSubscriptionService(repo, prov)

	n, err := svc.RenewDue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(0), n)
	assert.Equal(t, "", prov.lastCharge.token) // Charge не вызывался
	repo.AssertNotCalled(t, "MarkSuccessAndRenew", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestRenewDue_RecoversPriorPendingChargedYesterday(t *testing.T) {
	// Вчерашний Charge прошёл, активация упала: сегодня recovery должен
	// активировать БЕЗ нового списания.
	repo := new(mockSubRepo)
	prov := &fakeProvider{statusValue: "success"}
	pe := time.Now().Add(-48 * time.Hour)
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	priorOrderID := "t1:" + pe.Format(time.RFC3339) + ":" + yesterday
	repo.On("ListDueAutopay", mock.Anything, mock.Anything).
		Return([]models.DueSubscription{{TutorID: "t1", Plan: "monthly", CardToken: "tok1", PeriodEnd: pe}}, nil)
	repo.On("ListPeriodPayments", mock.Anything, mock.AnythingOfType("string")).
		Return([]models.SubscriptionPayment{{TutorID: "t1", OrderID: priorOrderID, Plan: "monthly", Amount: 10000, Status: "pending"}}, nil)
	repo.On("MarkSuccessAndRenew", mock.Anything, priorOrderID, priorOrderID, "t1", "monthly", mock.AnythingOfType("time.Time")).
		Return(true, nil)
	svc := service.NewSubscriptionService(repo, prov)

	n, err := svc.RenewDue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(1), n)
	assert.Equal(t, "", prov.lastCharge.token) // Charge НЕ вызывался
	repo.AssertNotCalled(t, "InsertPendingPayment", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestRenewDue_OneSubErrorDoesNotAbortTick(t *testing.T) {
	// Ошибка InsertPendingPayment у первого репетитора не должна хоронить второго.
	repo := new(mockSubRepo)
	prov := &fakeProvider{chargeID: "pp-mit"}
	pe := time.Now().Add(-time.Hour)
	repo.On("ListDueAutopay", mock.Anything, mock.Anything).
		Return([]models.DueSubscription{
			{TutorID: "t1", Plan: "monthly", CardToken: "tok1", PeriodEnd: pe},
			{TutorID: "t2", Plan: "monthly", CardToken: "tok2", PeriodEnd: pe},
		}, nil)
	repo.On("ListPeriodPayments", mock.Anything, mock.AnythingOfType("string")).Return(nil, nil)
	repo.On("InsertPendingPayment", mock.Anything, "t1", mock.AnythingOfType("string"), "monthly", service.PriceMonthly).
		Return(false, errors.New("db blip"))
	repo.On("InsertPendingPayment", mock.Anything, "t2", mock.AnythingOfType("string"), "monthly", service.PriceMonthly).
		Return(true, nil)
	repo.On("MarkSuccessAndRenew", mock.Anything, mock.AnythingOfType("string"), "pp-mit", "t2", "monthly", mock.AnythingOfType("time.Time")).
		Return(true, nil)
	svc := service.NewSubscriptionService(repo, prov)

	n, err := svc.RenewDue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(1), n)
}

func TestRenewDue_RecoveryAmbiguousStatus_SkipsCharge(t *testing.T) {
	// Вчерашняя pending-строка, CheckStatus упал: судьба списания неизвестна —
	// новое списание в этот тик запрещено.
	repo := new(mockSubRepo)
	prov := &fakeProvider{statusErr: errors.New("status timeout")}
	pe := time.Now().Add(-48 * time.Hour)
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	priorOrderID := "t1:" + pe.Format(time.RFC3339) + ":" + yesterday
	repo.On("ListDueAutopay", mock.Anything, mock.Anything).
		Return([]models.DueSubscription{{TutorID: "t1", Plan: "monthly", CardToken: "tok1", PeriodEnd: pe}}, nil)
	repo.On("ListPeriodPayments", mock.Anything, mock.AnythingOfType("string")).
		Return([]models.SubscriptionPayment{{TutorID: "t1", OrderID: priorOrderID, Plan: "monthly", Amount: 10000, Status: "pending"}}, nil)
	svc := service.NewSubscriptionService(repo, prov)

	n, err := svc.RenewDue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(0), n)
	assert.Equal(t, "", prov.lastCharge.token) // Charge НЕ вызывался
	repo.AssertNotCalled(t, "InsertPendingPayment", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "MarkPaymentFailed", mock.Anything, mock.Anything)
}

func TestRenewDue_ChargeErrAmbiguous_LeavesPending(t *testing.T) {
	// Charge упал и CheckStatus упал: строка должна остаться pending
	// (НЕ failed), чтобы recovery следующего тика могла её доразрулить.
	repo := new(mockSubRepo)
	prov := &fakeProvider{chargeErr: errors.New("charge timeout"), statusErr: errors.New("status timeout")}
	repo.On("ListDueAutopay", mock.Anything, mock.Anything).
		Return([]models.DueSubscription{{TutorID: "t1", Plan: "monthly", CardToken: "tok1", PeriodEnd: time.Now()}}, nil)
	repo.On("ListPeriodPayments", mock.Anything, mock.AnythingOfType("string")).Return(nil, nil)
	repo.On("InsertPendingPayment", mock.Anything, "t1", mock.AnythingOfType("string"), "monthly", service.PriceMonthly).Return(true, nil)
	svc := service.NewSubscriptionService(repo, prov)

	n, err := svc.RenewDue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(0), n)
	repo.AssertNotCalled(t, "MarkPaymentFailed", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "MarkSuccessAndRenew", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
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
