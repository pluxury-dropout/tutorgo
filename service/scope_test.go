package service_test

import (
	"context"
	"testing"
	"time"
	"tutorgo/models"
	"tutorgo/service"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Методы правок серии — на моках уроков и правил.

func (m *mockLessonRepo) Cancel(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockLessonRepo) ReassignToRule(ctx context.Context, lessonID, ruleID string, occurrenceDate time.Time) error {
	return m.Called(ctx, lessonID, ruleID, occurrenceDate).Error(0)
}
func (m *mockLessonRepo) DeleteFutureByRule(ctx context.Context, ruleID string, after time.Time) error {
	return m.Called(ctx, ruleID, after).Error(0)
}
func (m *mockRecurrenceRepo) Split(ctx context.Context, ruleID string, at time.Time, timeLocal string, duration int) (models.RecurrenceRule, error) {
	args := m.Called(ctx, ruleID, at, timeLocal, duration)
	return args.Get(0).(models.RecurrenceRule), args.Error(1)
}
func (m *mockRecurrenceRepo) UpdateTiming(ctx context.Context, ruleID, timeLocal string, duration int) error {
	return m.Called(ctx, ruleID, timeLocal, duration).Error(0)
}
func (m *mockRecurrenceRepo) SetEndsOn(ctx context.Context, ruleID string, endsOn time.Time) error {
	return m.Called(ctx, ruleID, endsOn).Error(0)
}

func seriesLesson() models.Lesson {
	ruleID := "rule-1"
	occ := date(2026, time.September, 8)
	return models.Lesson{
		ID: lessonID, CourseID: courseID, ScheduledAt: scheduledAt, DurationMinutes: 60,
		Status: "scheduled", RuleID: &ruleID, OccurrenceDate: &occ,
	}
}

func scopedSvc(lessonRepo *mockLessonRepo, ruleRepo *mockRecurrenceRepo) service.LessonService {
	return service.NewLessonService(lessonRepo, new(mockCourseRepo), new(mockPaymentRepo),
		service.NewRecurrenceService(ruleRepo))
}

// Перенос одного вхождения не трогает правило: иначе перетаскивание урока
// мышью незаметно сдвигало бы всю серию.
func TestLessonUpdate_ScopeOne(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	ruleRepo := new(mockRecurrenceRepo)
	svc := scopedSvc(lessonRepo, ruleRepo)

	lessonRepo.On("GetByIDForTutor", mock.Anything, lessonID, tutorID).Return(seriesLesson(), nil)
	lessonRepo.On("Update", mock.Anything, lessonID, updateLessonReq).Return(expectedLesson, nil)

	_, err := svc.Update(context.Background(), lessonID, updateLessonReq, tutorID, "one")

	require.NoError(t, err)
	ruleRepo.AssertNotCalled(t, "Split")
	ruleRepo.AssertNotCalled(t, "UpdateTiming")
}

// «Это и все следующие» разрезает правило: старое закрывается вчерашним днём,
// новое начинается с этого вхождения и материализуется заново.
func TestLessonUpdate_ScopeFollowing(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	ruleRepo := new(mockRecurrenceRepo)
	svc := scopedSvc(lessonRepo, ruleRepo)

	lesson := seriesLesson()
	newRule := models.RecurrenceRule{
		ID: "rule-2", Freq: "weekly", IntervalN: 1, TimeLocal: "22:00", TZ: "Asia/Almaty",
		DurationMinutes: 90, StartsOn: *lesson.OccurrenceDate, MaterializedUntil: *lesson.OccurrenceDate,
	}

	lessonRepo.On("GetByIDForTutor", mock.Anything, lessonID, tutorID).Return(lesson, nil)
	// Зона нового времени берётся из старого правила: время в правиле — стенные
	// часы, а запрос приходит в UTC.
	ruleRepo.On("GetByID", mock.Anything, "rule-1").
		Return(models.RecurrenceRule{ID: "rule-1", TZ: "Asia/Almaty"}, nil)
	ruleRepo.On("Split", mock.Anything, "rule-1", *lesson.OccurrenceDate, mock.Anything, 90).Return(newRule, nil)
	lessonRepo.On("Update", mock.Anything, lessonID, updateLessonReq).Return(expectedLesson, nil)
	lessonRepo.On("ReassignToRule", mock.Anything, lessonID, "rule-2", *lesson.OccurrenceDate).Return(nil)
	// Будущие вхождения старого правила уходят: их пересоздаст новое.
	lessonRepo.On("DeleteFutureByRule", mock.Anything, "rule-1", *lesson.OccurrenceDate).Return(nil)
	ruleRepo.On("GetByID", mock.Anything, "rule-2").Return(newRule, nil)
	ruleRepo.On("InsertOccurrences", mock.Anything, "rule-2", mock.Anything).Return(0, nil)
	ruleRepo.On("SetMaterializedUntil", mock.Anything, "rule-2", mock.Anything).Return(nil)

	_, err := svc.Update(context.Background(), lessonID, updateLessonReq, tutorID, "following")

	require.NoError(t, err)
	lessonRepo.AssertExpectations(t)
	ruleRepo.AssertExpectations(t)
}

// «Все» правит само правило и пересобирает будущие вхождения; прошедшие
// остаются как есть — они уже состоялись.
func TestLessonUpdate_ScopeAll(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	ruleRepo := new(mockRecurrenceRepo)
	svc := scopedSvc(lessonRepo, ruleRepo)

	lesson := seriesLesson()
	rule := models.RecurrenceRule{ID: "rule-1", TZ: "Asia/Almaty", StartsOn: date(2026, time.September, 1)}

	lessonRepo.On("GetByIDForTutor", mock.Anything, lessonID, tutorID).Return(lesson, nil)
	ruleRepo.On("UpdateTiming", mock.Anything, "rule-1", mock.Anything, 90).Return(nil)
	lessonRepo.On("Update", mock.Anything, lessonID, updateLessonReq).Return(expectedLesson, nil)
	lessonRepo.On("DeleteFutureByRule", mock.Anything, "rule-1", *lesson.OccurrenceDate).Return(nil)
	ruleRepo.On("GetByID", mock.Anything, "rule-1").Return(rule, nil)
	ruleRepo.On("InsertOccurrences", mock.Anything, "rule-1", mock.Anything).Return(0, nil)
	ruleRepo.On("SetMaterializedUntil", mock.Anything, "rule-1", mock.Anything).Return(nil)

	_, err := svc.Update(context.Background(), lessonID, updateLessonReq, tutorID, "all")

	require.NoError(t, err)
	ruleRepo.AssertNotCalled(t, "Split")
	lessonRepo.AssertExpectations(t)
}

// Отменённое вхождение остаётся строкой: удали его целиком — и ночная
// материализация создаст урок заново, потому что дата снова свободна.
func TestLessonDelete_ScopeOneCancelsOccurrence(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	ruleRepo := new(mockRecurrenceRepo)
	svc := scopedSvc(lessonRepo, ruleRepo)

	lessonRepo.On("GetByIDForTutor", mock.Anything, lessonID, tutorID).Return(seriesLesson(), nil)
	lessonRepo.On("Cancel", mock.Anything, lessonID).Return(nil)

	err := svc.Delete(context.Background(), lessonID, tutorID, "one")

	require.NoError(t, err)
	lessonRepo.AssertNotCalled(t, "Delete")
}

// Одиночный урок вне серии удаляется по-старому, без правил и отмен.
func TestLessonDelete_PlainLesson(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	ruleRepo := new(mockRecurrenceRepo)
	svc := scopedSvc(lessonRepo, ruleRepo)

	lessonRepo.On("GetByIDForTutor", mock.Anything, lessonID, tutorID).Return(expectedLesson, nil)
	lessonRepo.On("Delete", mock.Anything, lessonID).Return(nil)

	err := svc.Delete(context.Background(), lessonID, tutorID, "one")

	require.NoError(t, err)
	lessonRepo.AssertNotCalled(t, "Cancel")
}

func TestLessonDelete_ScopeAllDropsRule(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	ruleRepo := new(mockRecurrenceRepo)
	svc := scopedSvc(lessonRepo, ruleRepo)

	lesson := seriesLesson()
	lessonRepo.On("GetByIDForTutor", mock.Anything, lessonID, tutorID).Return(lesson, nil)
	// Сначала вхождения, потом правило: удаление правила обнуляет rule_id у
	// уроков (ON DELETE SET NULL), и найти их станет нечем.
	lessonRepo.On("DeleteFutureByRule", mock.Anything, "rule-1", mock.Anything).Return(nil)
	ruleRepo.On("Delete", mock.Anything, "rule-1").Return(nil)

	err := svc.Delete(context.Background(), lessonID, tutorID, "all")

	require.NoError(t, err)
	lessonRepo.AssertExpectations(t)
	ruleRepo.AssertExpectations(t)
}
