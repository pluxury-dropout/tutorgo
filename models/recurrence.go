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

// RecurrenceInput — правило в запросе на создание урока или события.
// Время начала и длительность здесь не спрашиваются: они и так есть у первого
// вхождения, а дублирующее поле рано или поздно разойдётся с ним.
type RecurrenceInput struct {
	Freq      string     `json:"freq"       validate:"required,oneof=daily weekly monthly"`
	IntervalN int        `json:"interval_n" validate:"omitempty,min=1"`
	ByWeekday []int      `json:"byweekday"  validate:"omitempty,max=7,dive,min=1,max=7"`
	TZ        string     `json:"tz"         validate:"required,max=64"`
	EndsOn    *time.Time `json:"ends_on"`
	MaxCount  *int       `json:"max_count"  validate:"omitempty,min=1,max=500"`
}
