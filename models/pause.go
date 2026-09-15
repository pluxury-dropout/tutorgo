package models

import "time"

// StudentPause — заморозка ученика с даты по дату включительно (спека, п. 6.9).
// Висит на ученике, а не на курсе: «уехал на месяц» закрывает все его предметы
// и группы одним действием.
type StudentPause struct {
	ID        string    `json:"id"`
	StudentID string    `json:"student_id"`
	StartsOn  time.Time `json:"starts_on"`
	EndsOn    time.Time `json:"ends_on"`
	Reason    *string   `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

// CreatePauseRequest — даты приходят полночью UTC, как paid_at у платежа.
// Порядок дат проверяет сервис: ошибка «конец раньше начала» понятнее, чем
// нарушение CHECK из базы.
type CreatePauseRequest struct {
	StartsOn time.Time `json:"starts_on" validate:"required"`
	EndsOn   time.Time `json:"ends_on"   validate:"required"`
	Reason   *string   `json:"reason"    validate:"omitempty,max=500"`
}
