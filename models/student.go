package models

import "time"

type Student struct {
	ID        string `json:"id"`
	TutorID   string `json:"tutor_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Notes     string `json:"notes"`
	Active    bool   `json:"active"`
}

type CreateStudentRequest struct {
	FirstName string `json:"first_name" validate:"required,min=2"`
	LastName  string `json:"last_name"  validate:"omitempty,min=2"`
	Phone     string `json:"phone"      validate:"omitempty,min=10"`
	Email     string `json:"email"      validate:"omitempty,email"`
	Notes     string `json:"notes"      validate:"omitempty,max=500"`
}

type UpdateStudentRequest struct {
	FirstName string `json:"first_name" validate:"required,min=2"`
	LastName  string `json:"last_name"  validate:"omitempty,min=2"`
	Phone     string `json:"phone"      validate:"omitempty,min=10"`
	Email     string `json:"email"      validate:"omitempty,email"`
	Notes     string `json:"notes"      validate:"omitempty,max=500"`
}

type AcceptInviteRequest struct {
	Token    string `json:"token"    validate:"required,uuid"`
	Username string `json:"username" validate:"required,min=3,max=32,alphanum"`
	Password string `json:"password" validate:"required,min=6"`
}

type StudentLoginRequest struct {
	Identifier string `json:"identifier" validate:"required"` // телефон или username
	Password   string `json:"password"   validate:"required,min=6"`
}

type StudentProfile struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
	Username  string `json:"username"`
}

type StudentChangePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required,min=6"`
	NewPassword string `json:"new_password" validate:"required,min=6"`
}

// StudentCourseSummary — строка предмета на карточке ученика: цена и баланс
// одним запросом, без похода на страницу курса (спека, п. 7.1). StartedAt и
// EndedAt нужны инлайн-правке цены на карточке ученика (п. 7.2, CoursePrice):
// UpdateCourseRequest.StartedAt обязателен (models/course.go:62), и без него
// сохранить цену будет нечем.
type StudentCourseSummary struct {
	CourseID        string        `json:"course_id"`
	Subject         string        `json:"subject"`
	IsGroup         bool          `json:"is_group"`
	PricePerCycle   float64       `json:"price_per_cycle"`
	LessonsPerCycle int           `json:"lessons_per_cycle"`
	StartedAt       time.Time     `json:"started_at"`
	EndedAt         *time.Time    `json:"ended_at"`
	Balance         CourseBalance `json:"balance"`
}

// StudentOverview — карточка ученика одним запросом вместо шести (спека,
// п. 7.1). Courses — только активные предметы, для отображения. PayableCourses
// — те же плюс архивные/ушедшие-из-группы с положительным долгом: форма
// оплаты обязана предложить и их, иначе такой долг невозможно погасить
// (спека, п. 7.0).
type StudentOverview struct {
	Student        Student          `json:"student"`
	Courses        []StudentCourseSummary `json:"courses"`
	PayableCourses []Course         `json:"payable_courses"`
	NextLesson     *CalendarLesson  `json:"next_lesson"`
	RecentLessons  []CalendarLesson `json:"recent_lessons"`
	Payments       []Payment        `json:"payments"`
	TotalOwed      float64          `json:"total_owed"`
}
