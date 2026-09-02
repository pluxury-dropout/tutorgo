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

func (m *mockEventRepo) Cancel(ctx context.Context, id, tutorID string) error {
	return m.Called(ctx, id, tutorID).Error(0)
}

func (m *mockEventRepo) ReassignToRule(ctx context.Context, eventID, ruleID string, occurrenceDate time.Time) error {
	return m.Called(ctx, eventID, ruleID, occurrenceDate).Error(0)
}

func (m *mockEventRepo) DeleteFutureByRule(ctx context.Context, ruleID string, after time.Time) error {
	return m.Called(ctx, ruleID, after).Error(0)
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

	// Удаление сперва читает событие: без rule_id не понять, вхождение это
	// серии (тогда отмена) или одиночное (тогда удаление).
	repo.On("GetByID", mock.Anything, "missing", "tutor-1").
		Return(models.Event{}, repository.ErrEventNotFound)

	err := svc.Delete(context.Background(), "missing", "tutor-1", "one")

	assert.ErrorIs(t, err, service.ErrNotFound)
	repo.AssertNotCalled(t, "Delete")
}

// Отменённое вхождение остаётся строкой-тумбстоуном: удали его целиком — и
// ночная материализация вернёт «спортзал» обратно, дата-то свободна.
func TestEventService_Delete_SeriesOccurrenceCancels(t *testing.T) {
	repo := new(mockEventRepo)
	svc := service.NewEventService(repo, nil)

	ruleID := "rule-1"
	occ := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	repo.On("GetByID", mock.Anything, "e1", "tutor-1").
		Return(models.Event{ID: "e1", RuleID: &ruleID, OccurrenceDate: &occ}, nil)
	repo.On("Cancel", mock.Anything, "e1", "tutor-1").Return(nil)

	err := svc.Delete(context.Background(), "e1", "tutor-1", "one")

	assert.NoError(t, err)
	repo.AssertNotCalled(t, "Delete")
	repo.AssertExpectations(t)
}
