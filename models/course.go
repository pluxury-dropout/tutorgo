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
	StudentID      string    `json:"student_id"`
	Subject        string    `json:"subject"`
	PricePerLesson float64   `json:"price_per_lesson"`
	StartedAt      time.Time `json:"started_at"`
	EndedAt        time.Time `json:"ended_at"`
}

type UpdateCourseRequest struct {
	Subject        string    `json:"subject"`
	PricePerLesson float64   `json:"price_per_lesson"`
	StartedAt      time.Time `json:"started_at"`
	EndedAt        time.Time `json:"ended_at"`
}
