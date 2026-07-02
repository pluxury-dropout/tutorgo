package service_test

import (
	"testing"
	"time"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/stretchr/testify/assert"
)

func ptr[T any](v T) *T { return &v }

func TestEffectiveState(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	end := now.Add(24 * time.Hour) // period_end через сутки
	past := now.Add(-24 * time.Hour)

	tests := []struct {
		name string
		sub  *models.Subscription
		want string
	}{
		{"nil row → blocked", nil, service.StateBlocked},
		{"grandfathered (period_end nil) → active",
			&models.Subscription{Grandfathered: true}, service.StateActive},
		{"before period_end → active",
			&models.Subscription{PeriodEnd: ptr(end)}, service.StateActive},
		{"exactly at period_end → grace",
			&models.Subscription{PeriodEnd: ptr(now)}, service.StateGrace},
		{"within grace → grace",
			&models.Subscription{PeriodEnd: ptr(past)}, service.StateGrace},
		{"past grace → blocked",
			&models.Subscription{PeriodEnd: ptr(now.Add(-8 * 24 * time.Hour))}, service.StateBlocked},
		{"non-grandfathered with nil period_end → blocked (safety)",
			&models.Subscription{PeriodEnd: nil}, service.StateBlocked},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, service.EffectiveState(tt.sub, now))
		})
	}
}
