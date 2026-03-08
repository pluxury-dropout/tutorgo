package models

import "time"

type Payment struct {
	ID           string    `json:"id"`
	CourseID     string    `json:"course_id"`
	Amount       float64   `json:"amount"`
	LessonsCount int       `json:"lessons_count"`
	PaidAt       time.Time `json:"paid_at"`
}

type CreatePaymentRequest struct {
	CourseID     string    `json:"course_id"     validate:"required,uuid"`
	Amount       float64   `json:"amount"        validate:"required,gt=0"`
	LessonsCount int       `json:"lessons_count" validate:"required,gt=0"`
	PaidAt       time.Time `json:"paid_at"       validate:"required"`
}
