package models

import "time"

type Payment struct {
	ID       string `json:"id"`
	CourseID string `json:"course_id"`
	// NULL — легаси-платёж группы, заведённый до миграции 039: восстановить
	// адресата неоткуда, в баланс конкретного ученика он не входит (спека, п. 3.4).
	StudentID    *string   `json:"student_id"`
	Amount       float64   `json:"amount"`
	LessonsCount int       `json:"lessons_count"`
	PaidAt       time.Time `json:"paid_at"`

	// Subject заполняют списки по репетитору (GetAllByTutor, GetAllByTutorPaged),
	// где платёж показывают вне контекста курса. StudentName — имя адресата в
	// списках и в истории курса; пуст у платежа без адресата.
	Subject     string  `json:"subject,omitempty"`
	StudentName *string `json:"student_name,omitempty"`
}

type CreatePaymentRequest struct {
	CourseID string `json:"course_id"     validate:"required,uuid"`
	// Обязателен в API, хотя колонка nullable: NULL — только легаси (спека, п. 3.4).
	StudentID    string    `json:"student_id"    validate:"required,uuid"`
	Amount       float64   `json:"amount"        validate:"required,gt=0"`
	LessonsCount int       `json:"lessons_count" validate:"required,gt=0"`
	PaidAt       time.Time `json:"paid_at"       validate:"required"`
}

// UpdatePaymentRequest несёт адресата: иначе легаси-платёж группы нечем
// починить из интерфейса (спека, п. 6.2).
type UpdatePaymentRequest struct {
	StudentID    string    `json:"student_id"    validate:"required,uuid"`
	Amount       float64   `json:"amount"        validate:"required,gt=0"`
	LessonsCount int       `json:"lessons_count" validate:"required,gt=0"`
	PaidAt       time.Time `json:"paid_at"       validate:"required"`
}
