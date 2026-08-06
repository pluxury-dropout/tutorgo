package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"html"
	"log/slog"
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
	repo   repository.PendingRegistrationRepository
	tutor  TutorService
	send   email.Sender
	appURL string
	log    *slog.Logger
}

func NewRegistrationService(repo repository.PendingRegistrationRepository, tutor TutorService, send email.Sender, appURL string, log *slog.Logger) RegistrationService {
	return &registrationService{repo: repo, tutor: tutor, send: send, appURL: appURL, log: log}
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

	// Welcome — тоже best-effort: аккаунт уже в БД, и завалить из-за него регистрацию
	// значит отправить человека на повторную попытку, где его встретит «email занят».
	if err := s.send(ctx, p.Email, welcomeSubject, welcomeBody(p.FirstName, s.appURL)); err != nil {
		s.log.Error("welcome email failed",
			slog.String("email", p.Email), slog.String("err", err.Error()))
	}
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

const (
	otpSubject     = "Код подтверждения Amida"
	welcomeSubject = "Добро пожаловать в Amida"
)

func otpBody(code string) string {
	return email.Layout("Код подтверждения", fmt.Sprintf(
		`<p style="margin:0 0 20px;font-size:15px;line-height:1.5;color:#3f3f46">Введите этот код, чтобы завершить регистрацию:</p>`+
			`<div style="background:#f4f4f5;border-radius:8px;padding:16px;text-align:center;`+
			`font-size:28px;font-weight:600;letter-spacing:0.2em;color:#18181b">%s</div>`+
			`<p style="margin:20px 0 0;font-size:13px;line-height:1.5;color:#71717a">Код действителен 10 минут. `+
			`Если вы не регистрировались — просто проигнорируйте письмо.</p>`,
		code))
}

// welcomeBody: имя приходит от пользователя и попадает в HTML — экранируем.
func welcomeBody(firstName, appURL string) string {
	greeting := "Здравствуйте!"
	if firstName != "" {
		greeting = fmt.Sprintf("Здравствуйте, %s!", html.EscapeString(firstName))
	}
	return email.Layout("Аккаунт создан", fmt.Sprintf(
		`<p style="margin:0 0 12px;font-size:15px;line-height:1.5;color:#3f3f46">%s</p>`+
			`<p style="margin:0 0 24px;font-size:15px;line-height:1.5;color:#3f3f46">`+
			`Всё готово: добавляйте учеников, ведите расписание, принимайте оплаты и проводите уроки `+
			`с видеосвязью и общей доской — в одном месте.</p>%s`,
		greeting, email.Button(appURL, "Перейти в кабинет")))
}
