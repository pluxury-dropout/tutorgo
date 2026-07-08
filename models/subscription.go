package models

import "time"

// Subscription — подписка репетитора на SaaS (1:1 с tutor).
type Subscription struct {
	TutorID       string     `json:"-"`
	Plan          *string    `json:"plan"`       // nil = пробный период
	PeriodEnd     *time.Time `json:"period_end"` // nil только для grandfathered
	Grandfathered bool       `json:"-"`
	Autopay       bool       `json:"-"`
	PendingPlan   *string    `json:"-"`
}

type Prices struct {
	Monthly  int    `json:"monthly"`
	Yearly   int    `json:"yearly"`
	Currency string `json:"currency"`
}

type SubscriptionStatus struct {
	State       string     `json:"state"`
	Plan        *string    `json:"plan"`
	PeriodEnd   *time.Time `json:"period_end"`
	Prices      Prices     `json:"prices"`
	Autopay     bool       `json:"autopay"`
	PendingPlan *string    `json:"pending_plan"`
}

type SubscriptionPlanRequest struct {
	Plan string `json:"plan" validate:"required,oneof=monthly yearly"`
}

// SubscriptionPayment — строка лога платежей (аудит + идемпотентность).
type SubscriptionPayment struct {
	TutorID string
	OrderID string
	Plan    string
	Amount  int
	Status  string
}

// DueSubscription — подписка, у которой пора списывать автопродление.
type DueSubscription struct {
	TutorID     string
	Plan        string
	PendingPlan *string
	CardToken   string
	PeriodEnd   time.Time
}
