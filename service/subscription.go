package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"tutorgo/models"
	"tutorgo/repository"
)

const (
	PriceMonthly = 10000
	PriceYearly  = 90000
	Currency     = "KZT"
)

type SubscriptionService interface {
	GetStatus(ctx context.Context, tutorID string) (models.SubscriptionStatus, error)
	State(ctx context.Context, tutorID string) (string, error)
	Checkout(ctx context.Context, tutorID, plan string) (string, error)
	Confirm(ctx context.Context, tutorID, plan string) error
	HandleWebhook(ctx context.Context, r *http.Request) error
	RenewDue(ctx context.Context) (int64, error)
	Cancel(ctx context.Context, tutorID string) error
	ChangePlan(ctx context.Context, tutorID, plan string) error
}

var ErrBadSignature = errors.New("bad webhook signature")

type subscriptionService struct {
	repo     repository.SubscriptionRepository
	provider PaymentProvider
}

func NewSubscriptionService(repo repository.SubscriptionRepository, provider PaymentProvider) SubscriptionService {
	return &subscriptionService{repo: repo, provider: provider}
}

func planAmount(plan string) int {
	if plan == "yearly" {
		return PriceYearly
	}
	return PriceMonthly
}

func planDays(plan string) int {
	if plan == "yearly" {
		return 365
	}
	return 30
}

func (s *subscriptionService) State(ctx context.Context, tutorID string) (string, error) {
	sub, err := s.repo.GetByTutor(ctx, tutorID)
	if err != nil {
		return "", err
	}
	return EffectiveState(sub, time.Now()), nil
}

func (s *subscriptionService) GetStatus(ctx context.Context, tutorID string) (models.SubscriptionStatus, error) {
	sub, err := s.repo.GetByTutor(ctx, tutorID)
	if err != nil {
		return models.SubscriptionStatus{}, err
	}
	st := models.SubscriptionStatus{
		State:  EffectiveState(sub, time.Now()),
		Prices: models.Prices{Monthly: PriceMonthly, Yearly: PriceYearly, Currency: Currency},
	}
	if sub != nil {
		st.Plan = sub.Plan
		st.PeriodEnd = sub.PeriodEnd
	}
	return st, nil
}

func (s *subscriptionService) Checkout(ctx context.Context, tutorID, plan string) (string, error) {
	orderID := uuid.NewString()
	amount := planAmount(plan)
	claimed, err := s.repo.InsertPendingPayment(ctx, tutorID, orderID, plan, amount)
	if err != nil {
		return "", err
	}
	if !claimed {
		return "", fmt.Errorf("duplicate order_id %s", orderID) // uuid-коллизия ~ невозможна
	}
	return s.provider.InitPayment(ctx, orderID, tutorID, plan, amount)
}

// Confirm — заглушка вместо вебхука провайдера: активирует подписку.
func (s *subscriptionService) Confirm(ctx context.Context, tutorID, plan string) error {
	days := 30
	if plan == "yearly" {
		days = 365
	}
	periodEnd := time.Now().Add(time.Duration(days) * 24 * time.Hour)
	return s.repo.Activate(ctx, tutorID, plan, periodEnd)
}

func (s *subscriptionService) HandleWebhook(ctx context.Context, r *http.Request) error {
	cb, err := s.provider.ParseCallback(r)
	if err != nil {
		return ErrBadSignature
	}
	pay, err := s.repo.GetPaymentByOrderID(ctx, cb.OrderID)
	if err != nil {
		return err
	}
	if pay == nil {
		return nil // неизвестный order_id — не наш, no-op (хендлер отдаст 200 + log)
	}
	if cb.Status != "success" {
		return s.repo.MarkPaymentFailed(ctx, cb.OrderID)
	}
	activated, err := s.repo.MarkPaymentSuccess(ctx, cb.OrderID, cb.ProviderPaymentID)
	if err != nil {
		return err
	}
	if !activated {
		return nil // повторный webhook — уже активировано
	}
	periodEnd := time.Now().AddDate(0, 0, planDays(pay.Plan))
	return s.repo.StartPaidPeriod(ctx, pay.TutorID, pay.Plan, cb.CardToken, periodEnd)
}

func (s *subscriptionService) RenewDue(ctx context.Context) (int64, error) {
	due, err := s.repo.ListDueAutopay(ctx, time.Now())
	if err != nil {
		return 0, err
	}
	today := time.Now().Format("2006-01-02")
	var renewed int64
	for _, d := range due {
		plan := d.Plan
		if d.PendingPlan != nil {
			plan = *d.PendingPlan // отложенная смена тарифа применяется на этом списании
		}
		orderID := fmt.Sprintf("%s:%s:%s", d.TutorID, d.PeriodEnd.Format(time.RFC3339), today)
		claimed, err := s.repo.InsertPendingPayment(ctx, d.TutorID, orderID, plan, planAmount(plan))
		if err != nil {
			return renewed, err
		}
		if !claimed {
			continue // сегодня уже пытались (другой инстанс / повтор тика)
		}
		ppid, err := s.provider.Charge(ctx, orderID, d.CardToken, planAmount(plan))
		if err != nil {
			// Reconciliation: g2g не дедупит, а Charge мог реально пройти (таймаут).
			// Спрашиваем статус; MarkFailed только при подтверждённом не-успехе.
			if st, sErr := s.provider.CheckStatus(ctx, orderID); sErr == nil && st == "success" {
				ppid = orderID // платёж прошёл; provider_payment_id недоступен — пишем orderID
			} else {
				_ = s.repo.MarkPaymentFailed(ctx, orderID) // остаётся в grace, ретрай завтра
				continue
			}
		}
		activated, err := s.repo.MarkPaymentSuccess(ctx, orderID, ppid)
		if err != nil {
			return renewed, err
		}
		if !activated {
			continue
		}
		newEnd := d.PeriodEnd.AddDate(0, 0, planDays(plan)) // аддитивно от старого period_end
		if err := s.repo.RenewPeriod(ctx, d.TutorID, plan, newEnd); err != nil {
			return renewed, err
		}
		renewed++
	}
	return renewed, nil
}

func (s *subscriptionService) Cancel(ctx context.Context, tutorID string) error {
	return s.repo.Cancel(ctx, tutorID)
}

func (s *subscriptionService) ChangePlan(ctx context.Context, tutorID, plan string) error {
	return s.repo.SetPendingPlan(ctx, tutorID, plan)
}
