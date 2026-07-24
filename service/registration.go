package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"tutorgo/email"
	"tutorgo/models"
	"tutorgo/repository"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

// Ошибки регистрации → HTTP-коды маппит handler.
var (
	ErrEmailTaken        = errors.New("email is already taken")
	ErrCodeExpired       = errors.New("code expired, register again")
	ErrTooManyAttempts   = errors.New("too many attempts, register again")
	ErrInvalidCode       = errors.New("invalid code")
	ErrResendCooldown    = errors.New("wait before requesting a new code")
	ErrNoPendingForEmail = errors.New("no pending registration for this email")
)

const (
	otpTTL      = 10 * time.Minute
	resendAfter = 60 * time.Second
	maxAttempts = 5
)

type RegistrationService interface {
	Start(ctx context.Context, req models.RegisterRequest) error
	Verify(ctx context.Context, email, code string) (models.Tutor, error)
	Resend(ctx context.Context, email string) error
}

type registrationService struct {
	repo  repository.PendingRegistrationRepository
	tutor TutorService
	send  email.Sender
}

func NewRegistrationService(repo repository.PendingRegistrationRepository, tutor TutorService, send email.Sender) RegistrationService {
	return &registrationService{repo: repo, tutor: tutor, send: send}
}

func (s *registrationService) Start(ctx context.Context, req models.RegisterRequest) error {
	// email уже занят реальным аккаунтом?
	if _, _, err := s.tutor.GetByEmail(ctx, req.Email); err == nil {
		return ErrEmailTaken
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	code := generateOTP()
	codeHash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	now := time.Now()
	if err := s.repo.Upsert(ctx, models.PendingRegistration{
		Email:        req.Email,
		PasswordHash: string(passwordHash),
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Phone:        req.Phone,
		CodeHash:     string(codeHash),
		ResendAt:     now.Add(resendAfter),
		ExpiresAt:    now.Add(otpTTL),
	}); err != nil {
		return err
	}
	return s.send(ctx, req.Email, otpSubject, otpBody(code))
}

func (s *registrationService) Verify(ctx context.Context, email, code string) (models.Tutor, error) {
	p, err := s.repo.GetByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && p.ExpiresAt.Before(time.Now())) {
		return models.Tutor{}, ErrCodeExpired
	}
	if err != nil {
		return models.Tutor{}, err
	}
	if p.Attempts >= maxAttempts {
		_ = s.repo.Delete(ctx, email)
		return models.Tutor{}, ErrTooManyAttempts
	}
	if bcrypt.CompareHashAndPassword([]byte(p.CodeHash), []byte(code)) != nil {
		_ = s.repo.IncrementAttempts(ctx, email)
		return models.Tutor{}, ErrInvalidCode
	}

	tutor, err := s.tutor.Register(ctx, models.CreateTutorRequest{
		Email:     p.Email,
		FirstName: p.FirstName,
		LastName:  p.LastName,
		Phone:     p.Phone,
	}, p.PasswordHash)
	if err != nil {
		return models.Tutor{}, err
	}
	_ = s.repo.Delete(ctx, email) // best-effort: аккаунт создан, остаток вычистит DeleteExpired
	return tutor, nil
}

func (s *registrationService) Resend(ctx context.Context, email string) error {
	p, err := s.repo.GetByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNoPendingForEmail
	}
	if err != nil {
		return err
	}
	if p.ResendAt.After(time.Now()) {
		return ErrResendCooldown
	}

	code := generateOTP()
	codeHash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	now := time.Now()
	p.CodeHash = string(codeHash)
	p.ResendAt = now.Add(resendAfter)
	p.ExpiresAt = now.Add(otpTTL)
	if err := s.repo.Upsert(ctx, p); err != nil { // Upsert сбрасывает attempts=0
		return err
	}
	return s.send(ctx, email, otpSubject, otpBody(code))
}

// generateOTP возвращает 6-значный код без modulo-bias: равномерно из [0, 1_000_000).
func generateOTP() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		panic(err) // crypto/rand не должен падать; лучше упасть, чем выдать предсказуемый код
	}
	return fmt.Sprintf("%06d", n.Int64())
}

const otpSubject = "Код подтверждения Amida"

func otpBody(code string) string {
	return fmt.Sprintf(
		`<p>Ваш код подтверждения: <strong style="font-size:20px">%s</strong></p>`+
			`<p>Код действителен 10 минут. Если вы не регистрировались — просто проигнорируйте письмо.</p>`,
		code)
}
