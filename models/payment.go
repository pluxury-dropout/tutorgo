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

// CourseDebt — долг ученика по одному курсу, строка выборки долгов. Наружу не
// отдаётся: сервис складывает строки в StudentDebt.
type CourseDebt struct {
	StudentID    string
	StudentName  string
	CourseID     string
	Subject      string
	LessonsOwed  int
	LessonPrice  float64 // пакет / N, округлено до тенге
	NextLessonAt *time.Time
}

// DebtByCourse — разбивка долга по предмету.
type DebtByCourse struct {
	CourseID    string  `json:"course_id"`
	Subject     string  `json:"subject"`
	LessonsOwed int     `json:"lessons_owed"`
	AmountOwed  float64 `json:"amount_owed"`
}

// StudentDebt — должник: долг по всем его курсам одной строкой (спека, п. 6.5).
type StudentDebt struct {
	StudentID    string         `json:"student_id"`
	StudentName  string         `json:"student_name"`
	LessonsOwed  int            `json:"lessons_owed"`
	AmountOwed   float64        `json:"amount_owed"`
	NextLessonAt *time.Time     `json:"next_lesson_at"` // когда напомнить
	Courses      []DebtByCourse `json:"courses"`
}
