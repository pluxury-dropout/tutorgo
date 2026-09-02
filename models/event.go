package models

import "time"

// Event — занятость репетитора, не привязанная к курсу: врач, спортзал,
// подготовка материалов, запланированный пробный урок.
type Event struct {
	ID              string    `json:"id"`
	TutorID         string    `json:"tutor_id"`
	Title           string    `json:"title"`
	Kind            string    `json:"kind"`
	StartsAt        time.Time `json:"starts_at"`
	DurationMinutes int       `json:"duration_minutes"`
	Color           string    `json:"color"`
	Location        string    `json:"location"`
	Notes           string    `json:"notes"`
}

// Recurrence превращает событие в серию — «спортзал каждый понедельник».
// RuleID и OccurrenceDate заполняет сервис, не клиент.
type CreateEventRequest struct {
	Recurrence     *RecurrenceInput `json:"recurrence" validate:"omitempty"`
	RuleID         string           `json:"-"`
	OccurrenceDate *time.Time       `json:"-"`

	Title           string    `json:"title"            validate:"required,max=200"`
	Kind            string    `json:"kind"             validate:"omitempty,oneof=personal work trial"`
	StartsAt        time.Time `json:"starts_at"        validate:"required"`
	DurationMinutes int       `json:"duration_minutes" validate:"required,gt=0"`
	Color           string    `json:"color"    validate:"omitempty,max=32"`
	Location        string    `json:"location" validate:"omitempty,max=200"`
	Notes           string    `json:"notes"    validate:"omitempty,max=2000"`
}

type UpdateEventRequest = CreateEventRequest
