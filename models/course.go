package models

import "time"

type Course struct {
	ID             string    `json:"id"`
	StudentID      string    `json:"student_id"`
	TutorID        string    `json:"tutor_id"`
	Subject        string    `json:"subject"`
	PricePerLesson float64   `json:"price_per_lesson"`
	StartedAt      time.Time `json:"started_at"`
	EndedAt        time.Time `json:"ended_at"`
}

type CreateCourseRequest struct {
	StudentID      string    `json:"student_id"       validate:"required,uuid"`
	Subject        string    `json:"subject"          validate:"required,min=2"`
	PricePerLesson float64   `json:"price_per_lesson" validate:"required,gt=0"`
	StartedAt      time.Time `json:"started_at"       validate:"required"`
	EndedAt        time.Time `json:"ended_at"         validate:"required"`
}

type UpdateCourseRequest struct {
	Subject        string    `json:"subject"          validate:"required,min=2"`
	PricePerLesson float64   `json:"price_per_lesson" validate:"required,gt=0"`
	StartedAt      time.Time `json:"started_at"       validate:"required"`
	EndedAt        time.Time `json:"ended_at"         validate:"required"`
}
