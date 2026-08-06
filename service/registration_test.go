package service_test

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"time"

	"tutorgo/email"
	"tutorgo/models"
	"tutorgo/repository"
	"tutorgo/service"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newSvc собирает сервис с тестовыми appURL и молчащим логгером.
func newSvc(repo repository.PendingRegistrationRepository, tutor service.TutorService, send email.Sender) service.RegistrationService {
	return service.NewRegistrationService(repo, tutor, send, "https://amida.test", slog.New(slog.DiscardHandler))
}

// --- in-memory fake репозиторий pending_registrations ---

type memPendingRepo struct{ m map[string]models.PendingRegistration }

func newMemPendingRepo() *memPendingRepo {
	return &memPendingRepo{m: map[string]models.PendingRegistration{}}
}

func (r *memPendingRepo) Upsert(_ context.Context, p models.PendingRegistration) error {
	p.Attempts = 0 // Upsert всегда сбрасывает attempts (как в SQL)
	r.m[p.Email] = p
	return nil
}
func (r *memPendingRepo) GetByEmail(_ context.Context, email string) (models.PendingRegistration, error) {
	p, ok := r.m[email]
	if !ok {
		return models.PendingRegistration{}, pgx.ErrNoRows
	}
	return p, nil
}
func (r *memPendingRepo) IncrementAttempts(_ context.Context, email string) error {
	p := r.m[email]
	p.Attempts++
	r.m[email] = p
	return nil
}
func (r *memPendingRepo) Delete(_ context.Context, email string) error {
	delete(r.m, email)
	return nil
}
func (r *memPendingRepo) DeleteExpired(_ context.Context) (int64, error) { return 0, nil }

// --- fake TutorService: встраиваем интерфейс, переопределяем только нужное ---

type fakeTutorSvc struct {
	service.TutorService
	emailExists bool
	registered  models.Tutor
	registerErr error
}

func (f *fakeTutorSvc) GetByEmail(_ context.Context, _ string) (string, string, error) {
	if f.emailExists {
		return "existing-id", "hash", nil
	}
	return "", "", pgx.ErrNoRows
}
func (f *fakeTutorSvc) Register(_ context.Context, _ models.CreateTutorRequest, _ string) (models.Tutor, error) {
	return f.registered, f.registerErr
}

// --- sender, ловящий сгенерированный код ---

// captureSender кладёт в captured последний увиденный 6-значный код.
// Письма без кода (welcome) пропускаются, иначе они затирали бы найденный OTP.
func captureSender(captured *string) func(context.Context, string, string, string) error {
	re := regexp.MustCompile(`\d{6}`)
	return func(_ context.Context, _, _, body string) error {
		if code := re.FindString(body); code != "" {
			*captured = code
		}
		return nil
	}
}

func startReq() models.RegisterRequest {
	return models.RegisterRequest{
		Email: "new@example.com", Password: "password123", FirstName: "Amir", LastName: "Bekov",
	}
}

func TestRegistration_Start_EmailTaken(t *testing.T) {
	svc := newSvc(newMemPendingRepo(), &fakeTutorSvc{emailExists: true}, captureSender(new(string)))
	err := svc.Start(context.Background(), startReq())
	assert.ErrorIs(t, err, service.ErrEmailTaken)
}

func TestRegistration_Start_HappyPath(t *testing.T) {
	repo := newMemPendingRepo()
	var code string
	svc := newSvc(repo, &fakeTutorSvc{}, captureSender(&code))

	require.NoError(t, svc.Start(context.Background(), startReq()))
	assert.Len(t, code, 6, "6-значный код ушёл письмом")
	_, ok := repo.m["new@example.com"]
	assert.True(t, ok, "pending сохранён")
}

func TestRegistration_Verify_Success(t *testing.T) {
	repo := newMemPendingRepo()
	var code string
	tutor := models.Tutor{ID: "new-id", Email: "new@example.com"}
	svc := newSvc(repo, &fakeTutorSvc{registered: tutor}, captureSender(&code))

	require.NoError(t, svc.Start(context.Background(), startReq()))
	got, err := svc.Verify(context.Background(), "new@example.com", code)
	require.NoError(t, err)
	assert.Equal(t, tutor, got)
	_, ok := repo.m["new@example.com"]
	assert.False(t, ok, "pending удалён после успеха")
}

func TestRegistration_Verify_WrongCode(t *testing.T) {
	repo := newMemPendingRepo()
	svc := newSvc(repo, &fakeTutorSvc{}, captureSender(new(string)))

	require.NoError(t, svc.Start(context.Background(), startReq()))
	_, err := svc.Verify(context.Background(), "new@example.com", "000000")
	assert.ErrorIs(t, err, service.ErrInvalidCode)
	assert.Equal(t, 1, repo.m["new@example.com"].Attempts, "попытка засчитана")
}

func TestRegistration_Verify_Expired(t *testing.T) {
	repo := newMemPendingRepo()
	svc := newSvc(repo, &fakeTutorSvc{}, captureSender(new(string)))

	// нет pending вовсе → трактуем как истёкший
	_, err := svc.Verify(context.Background(), "new@example.com", "123456")
	assert.ErrorIs(t, err, service.ErrCodeExpired)
}

func TestRegistration_Verify_TooManyAttempts(t *testing.T) {
	repo := newMemPendingRepo()
	repo.m["new@example.com"] = models.PendingRegistration{
		Email: "new@example.com", CodeHash: "x", Attempts: 5,
		ExpiresAt: time.Now().Add(time.Minute),
	}
	svc := newSvc(repo, &fakeTutorSvc{}, captureSender(new(string)))

	_, err := svc.Verify(context.Background(), "new@example.com", "123456")
	assert.ErrorIs(t, err, service.ErrTooManyAttempts)
	_, ok := repo.m["new@example.com"]
	assert.False(t, ok, "pending удалён при исчерпании попыток")
}

// sentEmail — одно перехваченное письмо; sender ниже копит их в срез.
type sentEmail struct{ to, subject, body string }

func recordingSender(out *[]sentEmail, failOn string) func(context.Context, string, string, string) error {
	return func(_ context.Context, to, subject, body string) error {
		*out = append(*out, sentEmail{to, subject, body})
		if failOn != "" && strings.Contains(subject, failOn) {
			return errors.New("resend down")
		}
		return nil
	}
}

func TestRegistration_Verify_SendsWelcome(t *testing.T) {
	var sent []sentEmail
	var code string
	repo := newMemPendingRepo()
	svc := newSvc(repo, &fakeTutorSvc{}, func(ctx context.Context, to, subj, body string) error {
		captureSender(&code)(ctx, to, subj, body)
		return recordingSender(&sent, "")(ctx, to, subj, body)
	})

	require.NoError(t, svc.Start(context.Background(), startReq()))
	_, err := svc.Verify(context.Background(), "new@example.com", code)
	require.NoError(t, err)

	require.Len(t, sent, 2, "первое письмо — код, второе — приветствие")
	assert.Equal(t, "new@example.com", sent[1].to)
	assert.Contains(t, sent[1].subject, "Добро пожаловать")
	assert.Contains(t, sent[1].body, "https://amida.test", "в приветствии есть ссылка на кабинет")
	assert.Contains(t, sent[1].body, "Amir", "обращение по имени")
}

func TestRegistration_Verify_SucceedsWhenWelcomeFails(t *testing.T) {
	var sent []sentEmail
	var code string
	repo := newMemPendingRepo()
	tutor := models.Tutor{ID: "new-id", Email: "new@example.com"}
	svc := newSvc(repo, &fakeTutorSvc{registered: tutor}, func(ctx context.Context, to, subj, body string) error {
		captureSender(&code)(ctx, to, subj, body)
		return recordingSender(&sent, "Добро пожаловать")(ctx, to, subj, body)
	})

	require.NoError(t, svc.Start(context.Background(), startReq()))
	got, err := svc.Verify(context.Background(), "new@example.com", code)

	require.NoError(t, err, "упавшее приветствие не отменяет уже созданный аккаунт")
	assert.Equal(t, tutor, got)
	_, ok := repo.m["new@example.com"]
	assert.False(t, ok, "pending всё равно удалён")
}

func TestRegistration_Resend_Cooldown(t *testing.T) {
	repo := newMemPendingRepo()
	repo.m["new@example.com"] = models.PendingRegistration{
		Email: "new@example.com", ResendAt: time.Now().Add(30 * time.Second),
	}
	svc := newSvc(repo, &fakeTutorSvc{}, captureSender(new(string)))

	err := svc.Resend(context.Background(), "new@example.com")
	assert.ErrorIs(t, err, service.ErrResendCooldown)
}
