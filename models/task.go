package models

import "time"

type Task struct {
	ID              string     `json:"id"`
	TutorID         string     `json:"tutor_id"`
	Title           string     `json:"title"`
	Status          string     `json:"status"`
	ScheduledAt     *time.Time `json:"scheduled_at"`
	DurationMinutes *int       `json:"duration_minutes"`
	CreatedAt       time.Time  `json:"created_at"`
}

type CreateTaskRequest struct {
	Title           string     `json:"title"            validate:"required,max=10000"`
	ScheduledAt     *time.Time `json:"scheduled_at"`
	DurationMinutes *int       `json:"duration_minutes" validate:"omitempty,gt=0"`
	Status          string     `json:"status"           validate:"omitempty,oneof=not_urgent urgent very_urgent done"`
}

type UpdateTaskRequest struct {
	Title           string     `json:"title"            validate:"required,max=10000"`
	ScheduledAt     *time.Time `json:"scheduled_at"`
	DurationMinutes *int       `json:"duration_minutes" validate:"omitempty,gt=0"`
	Status          string     `json:"status"           validate:"required,oneof=not_urgent urgent very_urgent done"`
}
