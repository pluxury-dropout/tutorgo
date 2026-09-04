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
func (m *mockLessonRepo) MarkOverride(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockEventRepo) MarkOverride(ctx context.Context, id, tutorID string) error {
	return m.Called(ctx, id, tutorID).Error(0)
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
// мышью незаметно сдвигало бы всю серию. Само вхождение при этом помечается
// вручную правленным — иначе ближайшее «это и все следующие» снесёт перенос
// (DeleteFutureByRule смотрит ровно на is_override) и материализация вернёт
// урок на место по расписанию.
func TestLessonUpdate_ScopeOne(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	ruleRepo := new(mockRecurrenceRepo)
	svc := scopedSvc(lessonRepo, ruleRepo)

	lessonRepo.On("GetByIDForTutor", mock.Anything, lessonID, tutorID).Return(seriesLesson(), nil)
	lessonRepo.On("Update", mock.Anything, lessonID, updateLessonReq).Return(expectedLesson, nil)
	lessonRepo.On("MarkOverride", mock.Anything, lessonID).Return(nil)

	_, err := svc.Update(context.Background(), lessonID, updateLessonReq, tutorID, "one")

	require.NoError(t, err)
	lessonRepo.AssertExpectations(t)
	ruleRepo.AssertNotCalled(t, "Split")
	ruleRepo.AssertNotCalled(t, "UpdateTiming")
}

// Отметка «проведён» и заметка — не отклонение от расписания: правило задаёт
// время, а не статус. Пометь их — и «это и все следующие» перестанет двигать
// урок, к которому просто дописали «принести учебник».
func TestLessonUpdate_StatusOnlyNoOverride(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	ruleRepo := new(mockRecurrenceRepo)
	svc := scopedSvc(lessonRepo, ruleRepo)

	lesson := seriesLesson()
	req := models.UpdateLessonRequest{
		ScheduledAt:     lesson.ScheduledAt,
		DurationMinutes: lesson.DurationMinutes,
		Status:          "completed",
		Notes:           "принести учебник",
	}

	lessonRepo.On("GetByIDForTutor", mock.Anything, lessonID, tutorID).Return(lesson, nil)
	lessonRepo.On("Update", mock.Anything, lessonID, req).Return(expectedLesson, nil)

	_, err := svc.Update(context.Background(), lessonID, req, tutorID, "one")

	require.NoError(t, err)
	lessonRepo.AssertNotCalled(t, "MarkOverride")
}

// Урок вне серии метить нечем: правило его не пересоздаёт, а лишний UPDATE
// в самом частом пути правки не нужен.
func TestLessonUpdate_PlainLessonNoOverride(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	ruleRepo := new(mockRecurrenceRepo)
	svc := scopedSvc(lessonRepo, ruleRepo)

	lessonRepo.On("GetByIDForTutor", mock.Anything, lessonID, tutorID).Return(expectedLesson, nil)
	lessonRepo.On("Update", mock.Anything, lessonID, updateLessonReq).Return(expectedLesson, nil)

	_, err := svc.Update(context.Background(), lessonID, updateLessonReq, tutorID, "one")

	require.NoError(t, err)
	lessonRepo.AssertNotCalled(t, "MarkOverride")
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
	// Правка всей серии не метит вхождение вручную правленным: иначе следующая
	// правка «все» обошла бы его стороной.
	lessonRepo.AssertNotCalled(t, "MarkOverride")
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

// У событий та же защита: «спортзал», перенесённый на вторник, должен пережить
// последующую правку всей серии.
func TestEventUpdate_ScopeOneMarksOverride(t *testing.T) {
	repo := new(mockEventRepo)
	svc := service.NewEventService(repo, nil)

	ruleID := "rule-1"
	occ := date(2026, time.September, 8)
	req := models.UpdateEventRequest{
		Title: "Спортзал", Kind: "personal", StartsAt: scheduledAt, DurationMinutes: 90,
	}

	repo.On("GetByID", mock.Anything, "e1", tutorID).
		Return(models.Event{ID: "e1", RuleID: &ruleID, OccurrenceDate: &occ}, nil)
	repo.On("Update", mock.Anything, "e1", tutorID, req).Return(models.Event{ID: "e1"}, nil)
	repo.On("MarkOverride", mock.Anything, "e1", tutorID).Return(nil)

	_, err := svc.Update(context.Background(), "e1", tutorID, req, "one")

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

// Переименование вхождения серии — не отклонение от расписания: логика та же,
// что у уроков, но живёт в другом сервисе, поэтому проверяется отдельно.
func TestEventUpdate_RenameOnlyNoOverride(t *testing.T) {
	repo := new(mockEventRepo)
	svc := service.NewEventService(repo, nil)

	ruleID := "rule-1"
	occ := date(2026, time.September, 8)
	current := models.Event{
		ID: "e1", Title: "Спортзал", Kind: "personal", StartsAt: scheduledAt,
		DurationMinutes: 90, RuleID: &ruleID, OccurrenceDate: &occ,
	}
	req := models.UpdateEventRequest{
		Title: "Зал с тренером", Kind: "personal", StartsAt: scheduledAt, DurationMinutes: 90,
	}

	repo.On("GetByID", mock.Anything, "e1", tutorID).Return(current, nil)
	repo.On("Update", mock.Anything, "e1", tutorID, req).Return(models.Event{ID: "e1"}, nil)

	_, err := svc.Update(context.Background(), "e1", tutorID, req, "one")

	require.NoError(t, err)
	repo.AssertNotCalled(t, "MarkOverride")
}

// Одиночное событие вне серии не метится.
func TestEventUpdate_PlainEventNoOverride(t *testing.T) {
	repo := new(mockEventRepo)
	svc := service.NewEventService(repo, nil)

	req := models.UpdateEventRequest{
		Title: "Врач", Kind: "personal", StartsAt: scheduledAt, DurationMinutes: 30,
	}

	repo.On("GetByID", mock.Anything, "e1", tutorID).Return(models.Event{ID: "e1"}, nil)
	repo.On("Update", mock.Anything, "e1", tutorID, req).Return(models.Event{ID: "e1"}, nil)

	_, err := svc.Update(context.Background(), "e1", tutorID, req, "one")

	require.NoError(t, err)
	repo.AssertNotCalled(t, "MarkOverride")
}
