package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"tutorgo/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockRefreshTokenRepo struct {
	mock.Mock
}

func (m *mockRefreshTokenRepo) Create(ctx context.Context, tutorID, token string, expiresAt time.Time) error {
	args := m.Called(ctx, tutorID, token, expiresAt)
	return args.Error(0)
}

func (m *mockRefreshTokenRepo) GetByToken(ctx context.Context, token string) (string, time.Time, error) {
	args := m.Called(ctx, token)
	return args.String(0), args.Get(1).(time.Time), args.Error(2)
}

func (m *mockRefreshTokenRepo) DeleteByToken(ctx context.Context, token string) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *mockRefreshTokenRepo) DeleteAllByTutorID(ctx context.Context, tutorID string) error {
	args := m.Called(ctx, tutorID)
	return args.Error(0)
}

func TestRefreshToken_Create_ReturnsNonEmptyToken(t *testing.T) {
	repo := new(mockRefreshTokenRepo)
	svc := service.NewRefreshTokenService(repo)

	repo.On("Create", mock.Anything, "tutor-1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).
		Return(nil)

	token, err := svc.Create(context.Background(), "tutor-1")

	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	repo.AssertExpectations(t)
}

func TestRefreshToken_Create_RepoError(t *testing.T) {
	repo := new(mockRefreshTokenRepo)
	svc := service.NewRefreshTokenService(repo)

	repo.On("Create", mock.Anything, "tutor-1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).
		Return(errors.New("db error"))

	token, err := svc.Create(context.Background(), "tutor-1")

	assert.Error(t, err)
	assert.Empty(t, token)
	repo.AssertExpectations(t)
}

func TestRefreshToken_Validate_ValidToken(t *testing.T) {
	repo := new(mockRefreshTokenRepo)
	svc := service.NewRefreshTokenService(repo)

	future := time.Now().Add(24 * time.Hour)
	repo.On("GetByToken", mock.Anything, "tok123").
		Return("tutor-1", future, nil)

	tutorID, err := svc.Validate(context.Background(), "tok123")

	assert.NoError(t, err)
	assert.Equal(t, "tutor-1", tutorID)
	repo.AssertExpectations(t)
}

func TestRefreshToken_Validate_Expired(t *testing.T) {
	repo := new(mockRefreshTokenRepo)
	svc := service.NewRefreshTokenService(repo)

	past := time.Now().Add(-1 * time.Hour)
	repo.On("GetByToken", mock.Anything, "tok123").
		Return("tutor-1", past, nil)

	tutorID, err := svc.Validate(context.Background(), "tok123")

	assert.Error(t, err)
	assert.Empty(t, tutorID)
	repo.AssertExpectations(t)
}

func TestRefreshToken_Validate_NotFound(t *testing.T) {
	repo := new(mockRefreshTokenRepo)
	svc := service.NewRefreshTokenService(repo)

	repo.On("GetByToken", mock.Anything, "tok123").
		Return("", time.Time{}, errors.New("not found"))

	tutorID, err := svc.Validate(context.Background(), "tok123")

	assert.Error(t, err)
	assert.Empty(t, tutorID)
	repo.AssertExpectations(t)
}

func TestRefreshToken_Revoke(t *testing.T) {
	repo := new(mockRefreshTokenRepo)
	svc := service.NewRefreshTokenService(repo)

	repo.On("DeleteByToken", mock.Anything, "tok123").Return(nil)

	err := svc.Revoke(context.Background(), "tok123")

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestRefreshToken_RevokeAll(t *testing.T) {
	repo := new(mockRefreshTokenRepo)
	svc := service.NewRefreshTokenService(repo)

	repo.On("DeleteAllByTutorID", mock.Anything, "tutor-1").Return(nil)

	err := svc.RevokeAll(context.Background(), "tutor-1")

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}
