package models

type RegisterRequest struct {
	Email     string `json:"email"      validate:"required,email"`
	Password  string `json:"password"   validate:"required,min=6"`
	FirstName string `json:"first_name" validate:"required,min=2"`
	LastName  string `json:"last_name"  validate:"required,min=2"`
	Phone     string `json:"phone"      validate:"omitempty,min=10"`
}

type LoginRequest struct {
	// TODO(human): choose validation tags for Email and Phone so that
	// exactly one of them is required (email OR phone login).
	Email    string `json:"email"    validate:"required_without=Phone,omitempty,email"`
	Phone    string `json:"phone"    validate:"required_without=Email,omitempty,min=10"`
	Password string `json:"password" validate:"required,min=6"`
}

type LoginResponse struct {
	Token string `json:"token"`
}
