package service

import (
	"time"

	"tutorgo/models"
)

const (
	StateActive  = "active"
	StateGrace   = "grace"
	StateBlocked = "blocked"

	GraceDays = 7
)

// EffectiveState вычисляет состояние доступа из period_end на момент now.
// Хранимого status нет — состояние всегда производное. См. spec.
func EffectiveState(sub *models.Subscription, now time.Time) string {
	if sub == nil {
		return StateBlocked // нет строки = блок
	}
	if sub.Grandfathered {
		return StateActive // short-circuit ДО сравнения дат (period_end == nil)
	}
	if sub.PeriodEnd == nil {
		return StateBlocked // не-grandfathered без даты = аномалия, безопасно блокируем
	}
	if now.Before(*sub.PeriodEnd) {
		return StateActive
	}
	if now.Before(sub.PeriodEnd.Add(GraceDays * 24 * time.Hour)) {
		return StateGrace
	}
	return StateBlocked
}
