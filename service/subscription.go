package service

import (
	"context"
	"fmt"
	"time"

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
	repo repository.SubscriptionRepository
}

func NewSubscriptionService(repo repository.SubscriptionRepository) SubscriptionService {
	return &subscriptionService{repo: repo}
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

// Checkout — заглушка. Позже: создание платёжной сессии у провайдера.
func (s *subscriptionService) Checkout(ctx context.Context, tutorID, plan string) (string, error) {
	return fmt.Sprintf("https://pay.example.invalid/checkout?tutor=%s&plan=%s", tutorID, plan), nil
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
