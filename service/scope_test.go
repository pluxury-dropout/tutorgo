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
func (m *mockLessonRepo) MarkOverride(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockEventRepo) MarkOverride(ctx context.Context, id, tutorID string) error {
	return m.Called(ctx, id, tutorID).Error(0)
}
func (m *mockRecurrenceRepo) Split(ctx context.Context, ruleID string, at time.Time, timeLocal string, duration int, byweekday []int) (models.RecurrenceRule, error) {
	args := m.Called(ctx, ruleID, at, timeLocal, duration, byweekday)
	return args.Get(0).(models.RecurrenceRule), args.Error(1)
}
func (m *mockRecurrenceRepo) UpdateTiming(ctx context.Context, ruleID, timeLocal string, duration int, byweekday []int) error {
	return m.Called(ctx, ruleID, timeLocal, duration, byweekday).Error(0)
}
func (m *mockRecurrenceRepo) DeleteFutureByRule(ctx context.Context, ruleID string, after time.Time, exceptID string) error {
	return m.Called(ctx, ruleID, after, exceptID).Error(0)
}
func (m *mockRecurrenceRepo) ReassignToRule(ctx context.Context, occurrenceID, ruleID string, occurrenceDate time.Time) error {
	return m.Called(ctx, occurrenceID, ruleID, occurrenceDate).Error(0)
}
func (m *mockRecurrenceRepo) SetEndsOn(ctx context.Context, ruleID string, endsOn time.Time) error {
	return m.Called(ctx, ruleID, endsOn).Error(0)
}
func (m *mockRecurrenceRepo) OccurrenceExists(ctx context.Context, ruleID string, date time.Time, exceptID string) (bool, error) {
	args := m.Called(ctx, ruleID, date, exceptID)
	return args.Bool(0), args.Error(1)
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
// (DeleteFutureByRule в recurrence-репозитории смотрит ровно на is_override) и
// материализация вернёт урок на место по расписанию.
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
	rule := models.RecurrenceRule{
		ID: "rule-1", Freq: "weekly", IntervalN: 1, ByWeekday: []int{2},
		TimeLocal: "17:00", TZ: "Asia/Almaty", DurationMinutes: 60,
		StartsOn: date(2026, time.September, 1),
		// Не «впритык» к RecurrenceHorizon (см. комментарий у retimeRule в
		// retime_test.go) — иначе тест ловит окно в пару дней у
		// Retime.Materialize() и гниёт вместе с календарём.
		MaterializedUntil: date(2026, time.October, 1),
	}
	newRule := models.RecurrenceRule{
		ID: "rule-2", Freq: "weekly", IntervalN: 1, TimeLocal: "22:00", TZ: "Asia/Almaty",
		DurationMinutes: 90, StartsOn: *lesson.OccurrenceDate, MaterializedUntil: *lesson.OccurrenceDate,
	}

	lessonRepo.On("GetByIDForTutor", mock.Anything, lessonID, tutorID).Return(lesson, nil)
	ruleRepo.On("GetByID", mock.Anything, "rule-1").Return(rule, nil)
	ruleRepo.On("Split", mock.Anything, "rule-1", mock.Anything, mock.Anything, 90, mock.Anything).Return(newRule, nil)
	lessonRepo.On("Update", mock.Anything, lessonID, mock.Anything).Return(expectedLesson, nil)
	ruleRepo.On("DeleteFutureByRule", mock.Anything, "rule-1", mock.Anything, lessonID).Return(nil)
	ruleRepo.On("ReassignToRule", mock.Anything, lessonID, "rule-2", mock.Anything).Return(nil)
	ruleRepo.On("GetByID", mock.Anything, "rule-2").Return(newRule, nil)
	ruleRepo.On("InsertOccurrences", mock.Anything, "rule-2", mock.Anything).Return(0, nil)
	ruleRepo.On("SetMaterializedUntil", mock.Anything, "rule-2", mock.Anything).Return(nil)

	_, err := svc.Update(context.Background(), lessonID, updateLessonReq, tutorID, "following")

	require.NoError(t, err)
	ruleRepo.AssertNotCalled(t, "UpdateTiming")
	lessonRepo.AssertNotCalled(t, "MarkOverride")
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
	rule := models.RecurrenceRule{
		ID: "rule-1", Freq: "weekly", IntervalN: 1, ByWeekday: []int{2},
		TimeLocal: "17:00", TZ: "Asia/Almaty", DurationMinutes: 60,
		StartsOn: date(2026, time.September, 1),
		// Не «впритык» к RecurrenceHorizon (см. комментарий у retimeRule в
		// retime_test.go) — иначе тест ловит окно в пару дней у
		// Retime.Materialize() и гниёт вместе с календарём.
		MaterializedUntil: date(2026, time.October, 1),
	}

	lessonRepo.On("GetByIDForTutor", mock.Anything, lessonID, tutorID).Return(lesson, nil)
	ruleRepo.On("GetByID", mock.Anything, "rule-1").Return(rule, nil)
	// Нормализация переносит запрос на день недели выбранной даты в неделе
	// вхождения: пятница 01.05 (updateLessonReq.ScheduledAt) при вторничном
	// вхождении 08.09 (seriesLesson().OccurrenceDate) даёт пятницу 11.09. Без
	// матчера на конкретное время строку NormalizeSeriesStart в updateSeries
	// можно удалить незаметно — тест не покраснеет.
	lessonRepo.On("Update", mock.Anything, lessonID, mock.MatchedBy(func(r models.UpdateLessonRequest) bool {
		return r.ScheduledAt.Equal(time.Date(2026, time.September, 11, 10, 0, 0, 0, time.UTC))
	})).Return(expectedLesson, nil)
	// Пре-флайт занятой даты: целевая пятница 11.09 у этой серии ничем не занята.
	ruleRepo.On("OccurrenceExists", mock.Anything, "rule-1", date(2026, time.September, 11), lessonID).Return(false, nil)
	ruleRepo.On("UpdateTiming", mock.Anything, "rule-1", mock.Anything, 90, mock.Anything).Return(nil)
	ruleRepo.On("SetMaterializedUntil", mock.Anything, "rule-1", mock.Anything).Return(nil)
	ruleRepo.On("DeleteFutureByRule", mock.Anything, "rule-1", mock.Anything, lessonID).Return(nil)
	ruleRepo.On("ReassignToRule", mock.Anything, lessonID, "rule-1", mock.Anything).Return(nil)
	ruleRepo.On("InsertOccurrences", mock.Anything, "rule-1", mock.Anything).Return(0, nil)

	_, err := svc.Update(context.Background(), lessonID, updateLessonReq, tutorID, "all")

	require.NoError(t, err)
	ruleRepo.AssertNotCalled(t, "Split")
	// Правка всей серии не метит вхождение вручную правленным: иначе следующая
	// правка «все» обошла бы его стороной.
	lessonRepo.AssertNotCalled(t, "MarkOverride")
	lessonRepo.AssertExpectations(t)
	// Без этой строки тест не доказывает ничего о главном дефекте: если убрать
	// из updateSeries весь вызов Retime, ruleRepo.On(...) остаются неиспользованными
	// ожиданиями, и только AssertExpectations на ruleRepo это ловит.
	ruleRepo.AssertExpectations(t)
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
	// Удаление будущих вхождений уехало в recurrence-репозиторий: правило
	// владеет и уроками, и событиями, и чистить их логично одним методом.
	ruleRepo.On("DeleteFutureByRule", mock.Anything, "rule-1", mock.Anything, "").Return(nil)
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

// «Изменить все» у события ходит через тот же Retime, что и у урока: раньше
// это была вторая копия логики, и баг со scope=all жил в обеих.
func TestEventUpdate_ScopeAllGoesThroughRetime(t *testing.T) {
	eventRepo := new(mockEventRepo)
	ruleRepo := new(mockRecurrenceRepo)
	svc := service.NewEventService(eventRepo, service.NewRecurrenceService(ruleRepo))

	ruleID := "rule-1"
	occ := date(2026, time.September, 8) // вторник
	rule := models.RecurrenceRule{
		ID: ruleID, Freq: "weekly", IntervalN: 1, ByWeekday: []int{2},
		TimeLocal: "17:00", TZ: "Asia/Almaty", DurationMinutes: 90,
		StartsOn: date(2026, time.September, 1),
		// Материализовано на месяц вперёд, а не «впритык» к RecurrenceHorizon
		// (см. комментарий у retimeRule в retime_test.go) — иначе тест ловит
		// окно в пару дней у Retime.Materialize() и гниёт вместе с календарём.
		MaterializedUntil: date(2026, time.October, 1),
	}
	current := models.Event{
		ID: "e1", Title: "Спортзал", Kind: "personal", StartsAt: scheduledAt,
		DurationMinutes: 90, RuleID: &ruleID, OccurrenceDate: &occ,
	}
	req := models.UpdateEventRequest{
		Title: "Спортзал", Kind: "personal",
		StartsAt:        time.Date(2026, time.September, 8, 5, 0, 0, 0, time.UTC),
		DurationMinutes: 90,
	}

	eventRepo.On("GetByID", mock.Anything, "e1", tutorID).Return(current, nil)
	ruleRepo.On("GetByID", mock.Anything, ruleID).Return(rule, nil)
	eventRepo.On("Update", mock.Anything, "e1", tutorID, mock.Anything).Return(models.Event{ID: "e1"}, nil)
	ruleRepo.On("UpdateTiming", mock.Anything, ruleID, "10:00", 90, []int{2}).Return(nil)
	// Ключевая проверка: граница откатывается на дату вхождения, иначе
	// Materialize выйдет сразу и будущее останется удалённым.
	ruleRepo.On("SetMaterializedUntil", mock.Anything, ruleID, occ).Return(nil).Once()
	ruleRepo.On("DeleteFutureByRule", mock.Anything, ruleID, occ.AddDate(0, 0, -1), "e1").Return(nil)
	ruleRepo.On("ReassignToRule", mock.Anything, "e1", ruleID, occ).Return(nil)
	ruleRepo.On("InsertOccurrences", mock.Anything, ruleID, mock.Anything).Return(0, nil)
	ruleRepo.On("SetMaterializedUntil", mock.Anything, ruleID, mock.Anything).Return(nil)

	_, err := svc.Update(context.Background(), "e1", tutorID, req, "all")

	require.NoError(t, err)
	ruleRepo.AssertExpectations(t)
	eventRepo.AssertNotCalled(t, "MarkOverride")
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
