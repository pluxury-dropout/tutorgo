package models

type Tutor struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
}

type CreateTutorRequest struct {
	Email     string `json:"email"      validate:"required,email"`
	FirstName string `json:"first_name" validate:"required,min=2"`
	LastName  string `json:"last_name"  validate:"required,min=2"`
	Phone     string `json:"phone"      validate:"omitempty,min=10"`
}

type UpdateTutorRequest struct {
	Email     string `json:"email"      validate:"required,email"`
	FirstName string `json:"first_name" validate:"required,min=2"`
	LastName  string `json:"last_name"  validate:"required,min=2"`
	Phone     string `json:"phone"      validate:"omitempty,min=10"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required,min=6"`
	NewPassword     string `json:"new_password"     validate:"required,min=6"`
}
