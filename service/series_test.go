package service_test

import (
	"context"
	"errors"
	"testing"
	"time"
	"tutorgo/models"
	"tutorgo/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func (m *mockRecurrenceRepo) Create(ctx context.Context, rule models.RecurrenceRule) (models.RecurrenceRule, error) {
	args := m.Called(ctx, rule)
	return args.Get(0).(models.RecurrenceRule), args.Error(1)
}
func (m *mockRecurrenceRepo) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

var weeklyInput = models.RecurrenceInput{
	Freq:      "weekly",
	IntervalN: 1,
	ByWeekday: []int{2, 4},
	TZ:        "Asia/Almaty",
}

// Серия — это правило плюс первый урок как шаблон: остальные вхождения
// материализация добирает уже из него.
func TestLessonCreate_WithRecurrence(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	ruleRepo := new(mockRecurrenceRepo)
	svc := service.NewLessonService(lessonRepo, courseRepo, new(mockPaymentRepo), service.NewRecurrenceService(ruleRepo))

	at := time.Date(2026, 9, 1, 17, 0, 0, 0, time.UTC)
	req := models.CreateLessonRequest{
		CourseID:        courseID,
		ScheduledAt:     at,
		DurationMinutes: 60,
		Recurrence:      &weeklyInput,
	}

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(expectedCourse, nil)
	// Правило собирается из первого урока: время и длительность не дублируются
	// в запросе, их незачем спрашивать дважды.
	ruleRepo.On("Create", mock.Anything, mock.MatchedBy(func(r models.RecurrenceRule) bool {
		return r.TutorID == tutorID && r.Freq == "weekly" &&
			r.DurationMinutes == 60 && r.TimeLocal == "22:00" && // 17:00 UTC = 22:00 в Алматы
			r.StartsOn.Format(time.DateOnly) == "2026-09-01"
	})).Return(models.RecurrenceRule{ID: "rule-1", TZ: "Asia/Almaty"}, nil)

	lessonRepo.On("Create", mock.Anything, mock.MatchedBy(func(r models.CreateLessonRequest) bool {
		return r.RuleID == "rule-1"
	})).Return(expectedLesson, nil)

	// Материализация идёт через тот же сервис, что и ночная джоба.
	ruleRepo.On("GetByID", mock.Anything, "rule-1").Return(models.RecurrenceRule{
		ID: "rule-1", Freq: "weekly", IntervalN: 1, ByWeekday: []int{2, 4},
		TimeLocal: "22:00", TZ: "Asia/Almaty", DurationMinutes: 60,
		StartsOn: date(2026, time.September, 1), MaterializedUntil: date(2026, time.September, 1),
	}, nil)
	ruleRepo.On("InsertOccurrences", mock.Anything, "rule-1", mock.Anything).Return(52, nil)
	ruleRepo.On("SetMaterializedUntil", mock.Anything, "rule-1", mock.Anything).Return(nil)

	lesson, err := svc.Create(context.Background(), req, tutorID)

	require.NoError(t, err)
	assert.Equal(t, expectedLesson, lesson)
	ruleRepo.AssertExpectations(t)
	lessonRepo.AssertExpectations(t)
}

// Без recurrence всё как раньше: правило не создаётся.
func TestLessonCreate_WithoutRecurrence(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	ruleRepo := new(mockRecurrenceRepo)
	svc := service.NewLessonService(lessonRepo, courseRepo, new(mockPaymentRepo), service.NewRecurrenceService(ruleRepo))

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(expectedCourse, nil)
	lessonRepo.On("Create", mock.Anything, createLessonReq).Return(expectedLesson, nil)

	_, err := svc.Create(context.Background(), createLessonReq, tutorID)

	require.NoError(t, err)
	ruleRepo.AssertNotCalled(t, "Create")
}

// Правило создалось, а первый урок — нет: правило-сирота никого не
// материализует (шаблона нет), но и висеть в базе ему незачем.
func TestLessonCreate_RecurrenceRollsBackOnLessonError(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	ruleRepo := new(mockRecurrenceRepo)
	svc := service.NewLessonService(lessonRepo, courseRepo, new(mockPaymentRepo), service.NewRecurrenceService(ruleRepo))

	req := createLessonReq
	req.Recurrence = &weeklyInput

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(expectedCourse, nil)
	ruleRepo.On("Create", mock.Anything, mock.Anything).Return(models.RecurrenceRule{ID: "rule-1"}, nil)
	lessonRepo.On("Create", mock.Anything, mock.Anything).Return(models.Lesson{}, errors.New("db is down"))
	ruleRepo.On("Delete", mock.Anything, "rule-1").Return(nil)

	_, err := svc.Create(context.Background(), req, tutorID)

	require.Error(t, err)
	ruleRepo.AssertCalled(t, "Delete", mock.Anything, "rule-1")
}
