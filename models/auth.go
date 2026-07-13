package models

import "time"

type RegisterRequest struct {
	Email     string `json:"email"      validate:"required,email"`
	Password  string `json:"password"   validate:"required,min=6"`
	FirstName string `json:"first_name" validate:"required,min=2"`
	LastName  string `json:"last_name"  validate:"required,min=2"`
	Phone     string `json:"phone"      validate:"omitempty,min=10"`
}

type LoginRequest struct {
	Email    string `json:"email"    validate:"required_without=Phone,omitempty,email"`
	Phone    string `json:"phone"    validate:"required_without=Email,omitempty,min=10"`
	Password string `json:"password" validate:"required,min=6"`
}

type LoginResponse struct {
	AccessToken string `json:"access_token"`
}

// VerifyRegistrationRequest — шаг 2 регистрации: подтверждение OTP-кода.
type VerifyRegistrationRequest struct {
	Email string `json:"email" validate:"required,email"`
	Code  string `json:"code"  validate:"required,len=6,number"`
}

// ResendRegistrationRequest — повторная отправка OTP-кода.
type ResendRegistrationRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// PendingRegistration — строка pending_registrations (аккаунт до подтверждения).
type PendingRegistration struct {
	Email        string
	PasswordHash string
	FirstName    string
	LastName     string
	Phone        string
	CodeHash     string
	Attempts     int
	ResendAt     time.Time
	ExpiresAt    time.Time
}
