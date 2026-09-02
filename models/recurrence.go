package models

import "time"

// RecurrenceRule — правило повторения, общее для уроков и событий.
// Вхождения материализуются в реальные строки (lessons/events) со ссылкой
// rule_id: id вхождения нужен посещаемости, доскам, комнатам и платежам,
// поэтому чистая развёртка правила на лету не подходит.
type RecurrenceRule struct {
	ID              string `json:"id"`
	TutorID         string `json:"tutor_id"`
	Freq            string `json:"freq"`       // daily | weekly | monthly
	IntervalN       int    `json:"interval_n"` // каждые N дней/недель/месяцев
	ByWeekday       []int  `json:"byweekday"`  // ISO: 1=Пн … 7=Вс, только для weekly
	TimeLocal       string `json:"time_local"` // «17:00» — стенные часы в зоне TZ
	TZ              string `json:"tz"`         // IANA, напр. Asia/Almaty
	DurationMinutes int    `json:"duration_minutes"`

	StartsOn          time.Time  `json:"starts_on"`
	EndsOn            *time.Time `json:"ends_on"`  // nil — бессрочно
	MaxCount          *int       `json:"max_count"` // nil — без ограничения
	MaterializedUntil time.Time  `json:"materialized_until"`
}
