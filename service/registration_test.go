package service_test

import (
	"context"
	"regexp"
	"testing"
	"time"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func captureSender(captured *string) func(context.Context, string, string, string) error {
	re := regexp.MustCompile(`\d{6}`)
	return func(_ context.Context, _, _, body string) error {
		*captured = re.FindString(body)
		return nil
	}
}

func startReq() models.RegisterRequest {
	return models.RegisterRequest{
		Email: "new@example.com", Password: "password123", FirstName: "Amir", LastName: "Bekov",
	}
}

func TestRegistration_Start_EmailTaken(t *testing.T) {
	svc := service.NewRegistrationService(newMemPendingRepo(), &fakeTutorSvc{emailExists: true}, captureSender(new(string)))
	err := svc.Start(context.Background(), startReq())
	assert.ErrorIs(t, err, service.ErrEmailTaken)
}

func TestRegistration_Start_HappyPath(t *testing.T) {
	repo := newMemPendingRepo()
	var code string
	svc := service.NewRegistrationService(repo, &fakeTutorSvc{}, captureSender(&code))

	require.NoError(t, svc.Start(context.Background(), startReq()))
	assert.Len(t, code, 6, "6-значный код ушёл письмом")
	_, ok := repo.m["new@example.com"]
	assert.True(t, ok, "pending сохранён")
}

func TestRegistration_Verify_Success(t *testing.T) {
	repo := newMemPendingRepo()
	var code string
	tutor := models.Tutor{ID: "new-id", Email: "new@example.com"}
	svc := service.NewRegistrationService(repo, &fakeTutorSvc{registered: tutor}, captureSender(&code))

	require.NoError(t, svc.Start(context.Background(), startReq()))
	got, err := svc.Verify(context.Background(), "new@example.com", code)
	require.NoError(t, err)
	assert.Equal(t, tutor, got)
	_, ok := repo.m["new@example.com"]
	assert.False(t, ok, "pending удалён после успеха")
}

func TestRegistration_Verify_WrongCode(t *testing.T) {
	repo := newMemPendingRepo()
	svc := service.NewRegistrationService(repo, &fakeTutorSvc{}, captureSender(new(string)))

	require.NoError(t, svc.Start(context.Background(), startReq()))
	_, err := svc.Verify(context.Background(), "new@example.com", "000000")
	assert.ErrorIs(t, err, service.ErrInvalidCode)
	assert.Equal(t, 1, repo.m["new@example.com"].Attempts, "попытка засчитана")
}

func TestRegistration_Verify_Expired(t *testing.T) {
	repo := newMemPendingRepo()
	svc := service.NewRegistrationService(repo, &fakeTutorSvc{}, captureSender(new(string)))

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
	svc := service.NewRegistrationService(repo, &fakeTutorSvc{}, captureSender(new(string)))

	_, err := svc.Verify(context.Background(), "new@example.com", "123456")
	assert.ErrorIs(t, err, service.ErrTooManyAttempts)
	_, ok := repo.m["new@example.com"]
	assert.False(t, ok, "pending удалён при исчерпании попыток")
}

func TestRegistration_Resend_Cooldown(t *testing.T) {
	repo := newMemPendingRepo()
	repo.m["new@example.com"] = models.PendingRegistration{
		Email: "new@example.com", ResendAt: time.Now().Add(30 * time.Second),
	}
	svc := service.NewRegistrationService(repo, &fakeTutorSvc{}, captureSender(new(string)))

	err := svc.Resend(context.Background(), "new@example.com")
	assert.ErrorIs(t, err, service.ErrResendCooldown)
}
