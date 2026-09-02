package service_test

import (
	"context"
	"testing"
	"time"
	"tutorgo/models"
	"tutorgo/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockRecurrenceRepo struct{ mock.Mock }

func (m *mockRecurrenceRepo) GetByID(ctx context.Context, id string) (models.RecurrenceRule, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(models.RecurrenceRule), args.Error(1)
}
func (m *mockRecurrenceRepo) DueForMaterialization(ctx context.Context, horizon time.Time) ([]models.RecurrenceRule, error) {
	args := m.Called(ctx, horizon)
	return args.Get(0).([]models.RecurrenceRule), args.Error(1)
}
func (m *mockRecurrenceRepo) SetMaterializedUntil(ctx context.Context, id string, until time.Time) error {
	return m.Called(ctx, id, until).Error(0)
}
func (m *mockRecurrenceRepo) InsertOccurrences(ctx context.Context, ruleID string, starts []time.Time) (int, error) {
	args := m.Called(ctx, ruleID, starts)
	return args.Int(0), args.Error(1)
}

func weeklyRule() models.RecurrenceRule {
	return models.RecurrenceRule{
		ID:                "rule-1",
		TutorID:           tutorID,
		Freq:              "weekly",
		IntervalN:         1,
		TimeLocal:         "17:00",
		TZ:                "Asia/Almaty",
		DurationMinutes:   60,
		StartsOn:          date(2026, time.September, 1),
		MaterializedUntil: date(2026, time.October, 1),
	}
}

// Догоняем горизонт: считаем вхождения от уже материализованной границы,
// вставляем и двигаем materialized_until.
func TestMaterialize_ExtendsToHorizon(t *testing.T) {
	repo := new(mockRecurrenceRepo)
	svc := service.NewRecurrenceService(repo)
	horizon := date(2026, time.November, 1)

	repo.On("GetByID", mock.Anything, "rule-1").Return(weeklyRule(), nil)
	// Вторники с 1 октября по 1 ноября: 6, 13, 20, 27 октября.
	repo.On("InsertOccurrences", mock.Anything, "rule-1", mock.MatchedBy(func(starts []time.Time) bool {
		return len(starts) == 4
	})).Return(4, nil)
	repo.On("SetMaterializedUntil", mock.Anything, "rule-1", horizon).Return(nil)

	n, err := svc.Materialize(context.Background(), "rule-1", horizon)

	require.NoError(t, err)
	assert.Equal(t, 4, n)
	repo.AssertExpectations(t)
}

// Правило, у которого горизонт уже дальше запрошенного, не трогаем: иначе
// ночная джоба каждый раз переписывала бы границу назад.
func TestMaterialize_AlreadyBeyondHorizon(t *testing.T) {
	repo := new(mockRecurrenceRepo)
	svc := service.NewRecurrenceService(repo)

	rule := weeklyRule()
	rule.MaterializedUntil = date(2027, time.January, 1)
	repo.On("GetByID", mock.Anything, "rule-1").Return(rule, nil)

	n, err := svc.Materialize(context.Background(), "rule-1", date(2026, time.November, 1))

	require.NoError(t, err)
	assert.Zero(t, n)
	repo.AssertNotCalled(t, "InsertOccurrences")
	repo.AssertNotCalled(t, "SetMaterializedUntil")
}

// Закончившееся правило вхождений не даёт, но границу всё равно двигаем —
// иначе оно вечно возвращается в выборку джобы.
func TestMaterialize_FinishedRule(t *testing.T) {
	repo := new(mockRecurrenceRepo)
	svc := service.NewRecurrenceService(repo)
	horizon := date(2026, time.November, 1)

	rule := weeklyRule()
	ends := date(2026, time.September, 20)
	rule.EndsOn = &ends
	repo.On("GetByID", mock.Anything, "rule-1").Return(rule, nil)
	repo.On("SetMaterializedUntil", mock.Anything, "rule-1", horizon).Return(nil)

	n, err := svc.Materialize(context.Background(), "rule-1", horizon)

	require.NoError(t, err)
	assert.Zero(t, n)
	repo.AssertNotCalled(t, "InsertOccurrences")
	repo.AssertExpectations(t)
}

// Одно битое правило не должно ронять всю ночную догрузку.
func TestExtendAll_SkipsBrokenRule(t *testing.T) {
	repo := new(mockRecurrenceRepo)
	svc := service.NewRecurrenceService(repo)
	horizon := date(2026, time.November, 1)

	broken := weeklyRule()
	broken.ID = "rule-broken"
	broken.TZ = "Mars/Olympus"
	good := weeklyRule()

	repo.On("DueForMaterialization", mock.Anything, horizon).Return([]models.RecurrenceRule{broken, good}, nil)
	repo.On("GetByID", mock.Anything, "rule-broken").Return(broken, nil)
	repo.On("GetByID", mock.Anything, "rule-1").Return(good, nil)
	repo.On("InsertOccurrences", mock.Anything, "rule-1", mock.Anything).Return(4, nil)
	repo.On("SetMaterializedUntil", mock.Anything, "rule-1", horizon).Return(nil)

	n, err := svc.ExtendAll(context.Background(), horizon)

	require.NoError(t, err)
	assert.Equal(t, 4, n)
	repo.AssertNotCalled(t, "SetMaterializedUntil", mock.Anything, "rule-broken", mock.Anything)
}
