package models

import "time"

type Course struct {
	ID              string     `json:"id"`
	StudentID       *string    `json:"student_id"`
	TutorID         string     `json:"tutor_id"`
	Subject         string     `json:"subject"`
	PricePerCycle   float64    `json:"price_per_cycle"`
	LessonsPerCycle int        `json:"lessons_per_cycle"`
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at"`
	IsActive        bool       `json:"is_active"`
}

type UpdateHomeworkRequest struct {
	Homework string `json:"homework" validate:"max=20000"`
}

// StudentHomework — ДЗ курса в кабинете ученика.
type StudentHomework struct {
	CourseID string `json:"course_id"`
	Subject  string `json:"subject"`
	Homework string `json:"homework"`
}

// StudentCourse — курс ученика в кабинете (для выбора доски и т.п.).
type StudentCourse struct {
	ID      string `json:"id"`
	Subject string `json:"subject"`
	TutorID string `json:"-"`
}

type CourseBalance struct {
	LessonsPaid      int `json:"lessons_paid"`
	LessonsCompleted int `json:"lessons_completed"`
	LessonsRemaining int `json:"lessons_remaining"`
}

// PricePerCycle допускает 0: бесплатный или ещё не оценённый курс — нормальный
// случай, цена ставится позже. StartedAt без required: пустое поле сервис
// заполняет сегодняшней датой.
type CreateCourseRequest struct {
	StudentID       *string    `json:"student_id"        validate:"omitempty,uuid"`
	Subject         string     `json:"subject"           validate:"required,min=2"`
	PricePerCycle   float64    `json:"price_per_cycle"   validate:"gte=0"`
	LessonsPerCycle int        `json:"lessons_per_cycle" validate:"required,min=1"`
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at"`
}

type UpdateCourseRequest struct {
	Subject         string     `json:"subject"           validate:"required,min=2"`
	PricePerCycle   float64    `json:"price_per_cycle"   validate:"required,gt=0"`
	LessonsPerCycle int        `json:"lessons_per_cycle" validate:"required,min=1"`
	StartedAt       time.Time  `json:"started_at"        validate:"required"`
	EndedAt         *time.Time `json:"ended_at"`
}
