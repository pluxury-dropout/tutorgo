package models

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
	LastName  string `json:"last_name"  validate:"required,min=2"`
	Phone     string `json:"phone"      validate:"omitempty,min=10"`
	Email     string `json:"email"      validate:"omitempty,email"`
	Notes     string `json:"notes"      validate:"omitempty,max=500"`
}

type UpdateStudentRequest struct {
	FirstName string `json:"first_name" validate:"required,min=2"`
	LastName  string `json:"last_name"  validate:"required,min=2"`
	Phone     string `json:"phone"      validate:"omitempty,min=10"`
	Email     string `json:"email"      validate:"omitempty,email"`
	Notes     string `json:"notes"      validate:"omitempty,max=500"`
}
