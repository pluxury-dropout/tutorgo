package service_test

import (
	"context"
	"testing"
	"time"

	"tutorgo/models"
	"tutorgo/repository"
	"tutorgo/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockEventRepo mocks repository.EventRepository
type mockEventRepo struct{ mock.Mock }

func (m *mockEventRepo) Create(ctx context.Context, tutorID string, req models.CreateEventRequest) (models.Event, error) {
	args := m.Called(ctx, tutorID, req)
	return args.Get(0).(models.Event), args.Error(1)
}

func (m *mockEventRepo) GetByID(ctx context.Context, id, tutorID string) (models.Event, error) {
	args := m.Called(ctx, id, tutorID)
	return args.Get(0).(models.Event), args.Error(1)
}

func (m *mockEventRepo) GetByRange(ctx context.Context, tutorID, from, to string) ([]models.Event, error) {
	args := m.Called(ctx, tutorID, from, to)
	return args.Get(0).([]models.Event), args.Error(1)
}

func (m *mockEventRepo) Update(ctx context.Context, id, tutorID string, req models.UpdateEventRequest) (models.Event, error) {
	args := m.Called(ctx, id, tutorID, req)
	return args.Get(0).(models.Event), args.Error(1)
}

func (m *mockEventRepo) Delete(ctx context.Context, id, tutorID string) error {
	args := m.Called(ctx, id, tutorID)
	return args.Error(0)
}

func (m *mockEventRepo) GetOccupiedInRange(ctx context.Context, tutorID, from, to, excludeType string, excludeID *string) ([]models.CalendarItem, error) {
	args := m.Called(ctx, tutorID, from, to, excludeType, excludeID)
	return args.Get(0).([]models.CalendarItem), args.Error(1)
}

func TestEventService_Create_DefaultsKind(t *testing.T) {
	repo := new(mockEventRepo)
	svc := service.NewEventService(repo, nil)

	repo.On("Create", mock.Anything, "tutor-1", mock.MatchedBy(func(r models.CreateEventRequest) bool {
		return r.Kind == "personal"
	})).Return(models.Event{ID: "e1"}, nil)

	_, err := svc.Create(context.Background(), "tutor-1", models.CreateEventRequest{
		Title: "Спортзал", StartsAt: time.Now(), DurationMinutes: 90,
	})

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestEventService_Delete_NotFound(t *testing.T) {
	repo := new(mockEventRepo)
	svc := service.NewEventService(repo, nil)

	repo.On("Delete", mock.Anything, "missing", "tutor-1").Return(repository.ErrEventNotFound)

	err := svc.Delete(context.Background(), "missing", "tutor-1")

	assert.ErrorIs(t, err, service.ErrNotFound)
}
