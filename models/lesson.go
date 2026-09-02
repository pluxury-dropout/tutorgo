package models

import "time"

// CreateBulkLessonRequest, как и CreateLessonRequest, принимает либо CourseID,
// либо пару StudentID + Subject — курс тогда создаётся неявно.
type CreateBulkLessonRequest struct {
	CourseID        string   `json:"course_id"        validate:"omitempty,uuid"`
	StudentID       string   `json:"student_id"       validate:"omitempty,uuid"`
	Subject         string   `json:"subject"          validate:"omitempty,min=2"`
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
	CyclePosition   *int      `json:"cycle_position,omitempty"`
	CycleSize       *int      `json:"cycle_size,omitempty"`

	// Заполнены только у вхождения правила: по ним фронт понимает, что правка
	// затрагивает серию, и спрашивает область.
	RuleID         *string    `json:"rule_id,omitempty"`
	OccurrenceDate *time.Time `json:"occurrence_date,omitempty"`
}

// CreateLessonRequest принимает либо CourseID (урок в существующем курсе), либо
// пару StudentID + Subject — тогда курс находится или создаётся по ней. Выбор
// «либо/либо» проверяет сервис: тегами validator такое не выражается.
type CreateLessonRequest struct {
	CourseID        string    `json:"course_id"        validate:"omitempty,uuid"`
	StudentID       string    `json:"student_id"       validate:"omitempty,uuid"`
	Subject         string    `json:"subject"          validate:"omitempty,min=2"`
	ScheduledAt     time.Time `json:"scheduled_at"     validate:"required"`
	DurationMinutes int       `json:"duration_minutes" validate:"required,gt=0"`
	Notes           string    `json:"notes"            validate:"omitempty,max=500"`

	// Recurrence превращает урок в серию: сам он становится первым вхождением
	// и шаблоном, остальные добирает материализация.
	Recurrence *RecurrenceInput `json:"recurrence" validate:"omitempty"`

	// Заполняются сервисом, не клиентом.
	RuleID         string     `json:"-"`
	OccurrenceDate *time.Time `json:"-"`
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
	Rank            *int      `json:"-"` // global rank within course, used to compute cycle position
	CyclePosition   *int      `json:"cycle_position,omitempty"`
	CycleSize       *int      `json:"cycle_size,omitempty"`
	Paid            *bool     `json:"paid,omitempty"`
}

type CurrentCycleInfo struct {
	CourseID    string    `json:"course_id"`
	Subject     string    `json:"subject"`
	StudentName *string   `json:"student_name"`
	Progress    int       `json:"progress"`
	CycleSize   int       `json:"cycle_size"`
	LastAt      time.Time `json:"last_at"`
}
