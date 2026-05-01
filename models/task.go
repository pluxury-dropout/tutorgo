package models

import "time"

type Task struct {
	ID              string    `json:"id"`
	TutorID         string    `json:"tutor_id"`
	Title           string    `json:"title"`
	ScheduledAt     time.Time `json:"scheduled_at"`
	DurationMinutes int       `json:"duration_minutes"`
	Done            bool      `json:"done"`
	CreatedAt       time.Time `json:"created_at"`
}

type CreateTaskRequest struct {
	Title           string    `json:"title"            validate:"required,max=200"`
	ScheduledAt     time.Time `json:"scheduled_at"     validate:"required"`
	DurationMinutes int       `json:"duration_minutes" validate:"required,gt=0"`
}

type UpdateTaskRequest struct {
	Title           string    `json:"title"            validate:"required,max=200"`
	ScheduledAt     time.Time `json:"scheduled_at"     validate:"required"`
	DurationMinutes int       `json:"duration_minutes" validate:"required,gt=0"`
	Done            bool      `json:"done"`
}
