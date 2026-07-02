package models

import "time"

// Subscription — подписка репетитора на SaaS (1:1 с tutor).
type Subscription struct {
	TutorID       string     `json:"-"`
	Plan          *string    `json:"plan"`       // nil = пробный период
	PeriodEnd     *time.Time `json:"period_end"` // nil только для grandfathered
	Grandfathered bool       `json:"-"`
}
