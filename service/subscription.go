package service

import (
	"context"
	"fmt"
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
}

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
