package models

import "time"

type CreateBulkLessonRequest struct {
	CourseID        string   `json:"course_id"        validate:"required,uuid"`
	ScheduledAts    []string `json:"scheduled_ats"    validate:"required,min=1"`
	DurationMinutes int      `json:"duration_minutes" validate:"required,gt=0"`
	Notes           string   `json:"notes"            validate:"omitempty,max=500"`
}

type Lesson struct {
	ID              string    `json:"id"`
	CourseID        string    `json:"course_id"`
	ScheduledAt     time.Time `json:"scheduled_at"`
	DurationMinutes int       `json:"duration_minutes"`
	Status          string    `json:"status"`
	Notes           string    `json:"notes"`
	SeriesID        *string   `json:"series_id,omitempty"`
}

type CreateLessonRequest struct {
	CourseID        string    `json:"course_id"        validate:"required,uuid"`
	ScheduledAt     time.Time `json:"scheduled_at"     validate:"required"`
	DurationMinutes int       `json:"duration_minutes" validate:"required,gt=0"`
	Notes           string    `json:"notes"            validate:"omitempty,max=500"`
}

type UpdateLessonRequest struct {
	ScheduledAt     time.Time `json:"scheduled_at"     validate:"required"`
	DurationMinutes int       `json:"duration_minutes" validate:"required,gt=0"`
	Status          string    `json:"status"           validate:"omitempty,oneof=scheduled completed cancelled missed"`
	Notes           string    `json:"notes"            validate:"omitempty,max=500"`
}

// UpdateSeriesRequest patches all lessons in a series. All fields are optional.
// NewTime format: "HH:MM" (UTC). FromDate: RFC3339 date used as lower bound.
type UpdateSeriesRequest struct {
	FromDate        *string `json:"from_date"`
	NewTime         *string `json:"new_time"         validate:"omitempty"`
	DurationMinutes *int    `json:"duration_minutes" validate:"omitempty,gt=0"`
	Notes           *string `json:"notes"            validate:"omitempty,max=500"`
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
	SeriesID        *string   `json:"series_id,omitempty"`
}
