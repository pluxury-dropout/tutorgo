package models

import "time"

type Payment struct {
	ID           string    `json:"id"`
	CourseID     string    `json:"course_id"`
	Amount       float64   `json:"amount"`
	LessonsCount int       `json:"lessons_count"`
	PaidAt       time.Time `json:"paid_at"`

	// Заполняются только списочными выборками по репетитору (GetAllByTutor,
	// GetAllByTutorPaged) — там, где платёж показывают вне контекста курса и
	// нужно понимать, кто заплатил. StudentName пуст у групповых курсов.
	Subject     string  `json:"subject,omitempty"`
	StudentName *string `json:"student_name,omitempty"`
}

type CreatePaymentRequest struct {
	CourseID     string    `json:"course_id"     validate:"required,uuid"`
	Amount       float64   `json:"amount"        validate:"required,gt=0"`
	LessonsCount int       `json:"lessons_count" validate:"required,gt=0"`
	PaidAt       time.Time `json:"paid_at"       validate:"required"`
}

type UpdatePaymentRequest struct {
	Amount       float64   `json:"amount"        validate:"required,gt=0"`
	LessonsCount int       `json:"lessons_count" validate:"required,gt=0"`
	PaidAt       time.Time `json:"paid_at"       validate:"required"`
}
