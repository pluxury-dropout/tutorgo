package models

import "time"

type Lesson struct {
	ID              string    `json:"id"`
	CourseID        string    `json:"course_id"`
	ScheduledAt     time.Time `json:"scheduled_at"`
	DurationMinutes int       `json:"duration_minutes"`
	Status          string    `json:"status"`
	Notes           string    `json:"notes"`
}

type CreateLessonRequest struct {
	CourseID        string    `json:"course_id"         validate:"required,uuid"`
	ScheduledAt     time.Time `json:"scheduled_at"      validate:"required"`
	DurationMinutes int       `json:"duration_minutes"  validate:"required,gt=0"`
	Notes           string    `json:"notes"             validate:"omitempty,max=500"`
}

type UpdateLessonRequest struct {
	ScheduledAt     time.Time `json:"scheduled_at"      validate:"required"`
	DurationMinutes int       `json:"duration_minutes"  validate:"required,gt=0"`
	Status          string    `json:"status"            validate:"omitempty,oneof=scheduled completed cancelled missed"`
	Notes           string    `json:"notes"             validate:"omitempty,max=500"`
}

type CalendarLesson struct {
	ID              string    `json:"id"`
	CourseID        string    `json:"course_id"`
	ScheduledAt     time.Time `json:"scheduled_at"`
	DurationMinutes int       `json:"duration_minutes"`
	Status          string    `json:"status"`
	Notes           string    `json:"notes"`
	Subject         string    `json:"subject"`
	StudentName     *string   `json:"student_name"`
	IsGroup         bool      `json:"is_group"`
}
