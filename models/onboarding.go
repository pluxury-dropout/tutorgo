package models

import "time"

// OnboardingSchedule — «когда заниматься» в терминах формы: дни недели плюс
// стенное время. В правило повторения это переводит сервис.
type OnboardingSchedule struct {
	ByWeekday       []int      `json:"byweekday"        validate:"omitempty,max=7,dive,min=1,max=7"`
	TimeLocal       string     `json:"time_local"       validate:"required"` // «17:00»
	TZ              string     `json:"tz"               validate:"required,max=64"`
	DurationMinutes int        `json:"duration_minutes" validate:"required,gt=0"`
	StartsOn        time.Time  `json:"starts_on"        validate:"required"`
	EndsOn          *time.Time `json:"ends_on"`
}

// OnboardingStudentRequest — весь первый ученик одним сабмитом. Обязательно
// только имя: без subject заводится один ученик, без schedule — ученик и курс
// без уроков. Остальное дозаполняется потом на карточке.
type OnboardingStudentRequest struct {
	FirstName       string              `json:"first_name"        validate:"required,min=2"`
	Phone           string              `json:"phone"             validate:"omitempty,min=10"`
	Subject         string              `json:"subject"           validate:"omitempty,min=2"`
	PricePerCycle   float64             `json:"price_per_cycle"   validate:"omitempty,gte=0"`
	LessonsPerCycle int                 `json:"lessons_per_cycle" validate:"omitempty,min=1"`
	Schedule        *OnboardingSchedule `json:"schedule"`
}

type OnboardingResult struct {
	Student        Student `json:"student"`
	Course         *Course `json:"course"`
	LessonsCreated int     `json:"lessons_created"`
}
