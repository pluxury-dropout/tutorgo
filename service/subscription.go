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
	periodEnd := time.Now().AddDate(0, 0, planDays(pay.Plan))
	// Атомарно: платёж → success + активация периода. Ошибка → rollback обоих,
	// платёж остаётся pending, хендлер отдаст 5xx → провайдер передоставит.
	// false = повторный webhook (уже активировано) → no-op.
	_, err = s.repo.MarkSuccessAndStartPeriod(ctx, cb.OrderID, cb.ProviderPaymentID, pay.TutorID, pay.Plan, cb.CardToken, periodEnd)
	return err
}

func (s *subscriptionService) RenewDue(ctx context.Context) (int64, error) {
	due, err := s.repo.ListDueAutopay(ctx, time.Now())
	if err != nil {
		return 0, err
	}
	today := time.Now().Format("2006-01-02")
	var renewed int64
	for _, d := range due {
		periodKey := fmt.Sprintf("%s:%s:", d.TutorID, d.PeriodEnd.Format(time.RFC3339))
		orderID := periodKey + today

		// Recovery: незавершённые попытки ПРОШЛЫХ дней этого периода.
		// Charge мог пройти, а активация упасть — тогда списывать снова нельзя.
		prior, err := s.repo.ListPeriodPayments(ctx, periodKey)
		if err != nil {
			continue // без картины периода не списываем; ретрай следующим тиком
		}
		recovered := false
		for _, p := range prior {
			if p.OrderID == orderID || p.Status != "pending" {
				continue // сегодняшняя строка (может быть в полёте у другого инстанса) или терминальная
			}
			st, sErr := s.provider.CheckStatus(ctx, p.OrderID)
			if sErr == nil && st == "success" {
				// Деньги уже взяты (по плану p.Plan) — активируем без нового списания.
				newEnd := d.PeriodEnd.AddDate(0, 0, planDays(p.Plan))
				if ok, rErr := s.repo.MarkSuccessAndRenew(ctx, p.OrderID, p.OrderID, d.TutorID, p.Plan, newEnd); rErr == nil && ok {
					renewed++
				}
				recovered = true
				break
			}
			if sErr == nil && st == "failed" {
				_ = s.repo.MarkPaymentFailed(ctx, p.OrderID) // гигиена: прошлый день не в полёте
			}
		}
		if recovered {
			continue
		}

		plan := d.Plan
		if d.PendingPlan != nil {
			plan = *d.PendingPlan // отложенная смена тарифа применяется на этом списании
		}
		claimed, err := s.repo.InsertPendingPayment(ctx, d.TutorID, orderID, plan, planAmount(plan))
		if err != nil || !claimed {
			continue // ошибка → ретрай следующим тиком; !claimed → сегодня уже пытались
		}
		ppid, err := s.provider.Charge(ctx, orderID, d.CardToken, planAmount(plan))
		if err != nil {
			// Reconciliation: g2g не дедупит, Charge мог пройти (таймаут).
			if st, sErr := s.provider.CheckStatus(ctx, orderID); sErr == nil && st == "success" {
				ppid = orderID // прошёл; provider_payment_id недоступен — пишем orderID
			} else {
				_ = s.repo.MarkPaymentFailed(ctx, orderID) // остаётся в grace, ретрай завтра
				continue
			}
		}
		newEnd := d.PeriodEnd.AddDate(0, 0, planDays(plan))
		if ok, rErr := s.repo.MarkSuccessAndRenew(ctx, orderID, ppid, d.TutorID, plan, newEnd); rErr == nil && ok {
			renewed++
		}
		// rErr != nil: платёж остался pending — recovery следующего тика доразрулит через CheckStatus.
	}
	return renewed, nil
}

func (s *subscriptionService) Cancel(ctx context.Context, tutorID string) error {
	return s.repo.Cancel(ctx, tutorID)
}

func (s *subscriptionService) ChangePlan(ctx context.Context, tutorID, plan string) error {
	return s.repo.SetPendingPlan(ctx, tutorID, plan)
}
