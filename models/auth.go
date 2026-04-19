package models

type RegisterRequest struct {
	Email     string `json:"email"      validate:"required,email"`
	Password  string `json:"password"   validate:"required,min=6"`
	FirstName string `json:"first_name" validate:"required,min=2"`
	LastName  string `json:"last_name"  validate:"required,min=2"`
	Phone     string `json:"phone"      validate:"omitempty,min=10"`
}

type LoginRequest struct {
	Email    string `json:"email"    validate:"omitempty,email"`
	Password string `json:"password" validate:"omitempty, min=10"`
	Phone    string `json:"phone"    validate:"required"`
}

type LoginResponse struct {
	Token string `json:"token"`
}
