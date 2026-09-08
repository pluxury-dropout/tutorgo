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

const occID = "lesson-uuid-1"

// Правило: еженедельно вт+чт, 17:00 Алматы, горизонт уже дотянут на месяц вперёд.
func retimeRule() models.RecurrenceRule {
	return models.RecurrenceRule{
		ID: "rule-1", TutorID: tutorID, Freq: "weekly", IntervalN: 1,
		ByWeekday: []int{2, 4}, TimeLocal: "17:00", TZ: "Asia/Almaty",
		DurationMinutes: 60,
		StartsOn:        date(2026, time.September, 1),
		// Ровно тот случай, из-за которого Materialize раньше выходил сразу и
		// оставлял окно между «сегодня» и горизонтом пустым: граница
		// материализации в будущем, но заведомо ближе, чем RecurrenceHorizon
		// (6 месяцев) от даты запуска теста — иначе Retime.Materialize() в конце
		// не находит вхождений и InsertOccurrences не вызывается. Дата зашита
		// константой, а не «впритык» к горизонту (было 2027-03-05: тест ловил
		// окно в несколько дней и падал вместе с ходом календаря) — с запасом
		// в разы она остаётся верной ещё очень долго.
		MaterializedUntil: date(2026, time.October, 1),
	}
}

// «Изменить все» откатывает границу материализации на дату разреза — иначе
// Materialize выйдет по !MaterializedUntil.Before(horizon) и будущие вхождения,
// только что удалённые, никто не пересоздаст.
func TestRetime_ScopeAllRewindsWatermark(t *testing.T) {
	repo := new(mockRecurrenceRepo)
	svc := service.NewRecurrenceService(repo)
	rule := retimeRule()
	from := date(2026, time.September, 10)                                // четверг
	newStart := time.Date(2026, time.September, 10, 5, 0, 0, 0, time.UTC) // 10:00 Алматы

	repo.On("UpdateTiming", mock.Anything, "rule-1", "10:00", 90, []int{2, 4}).Return(nil)
	repo.On("SetMaterializedUntil", mock.Anything, "rule-1", from).Return(nil)
	repo.On("DeleteFutureByRule", mock.Anything, "rule-1", from.AddDate(0, 0, -1), occID).Return(nil)
	repo.On("ReassignToRule", mock.Anything, occID, "rule-1", from).Return(nil)
	repo.On("GetByID", mock.Anything, "rule-1").Return(rule, nil)
	repo.On("InsertOccurrences", mock.Anything, "rule-1", mock.Anything).Return(0, nil)
	repo.On("SetMaterializedUntil", mock.Anything, "rule-1", mock.Anything).Return(nil)

	err := svc.Retime(context.Background(), rule, occID, from, newStart, 90, "all")

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

// Порядок обязателен: откат границы идёт ДО удаления. Обрыв между ними оставит
// правило в выборке ночной джобы, обратный порядок — дыру навсегда.
func TestRetime_RewindHappensBeforeDelete(t *testing.T) {
	repo := new(mockRecurrenceRepo)
	svc := service.NewRecurrenceService(repo)
	rule := retimeRule()
	from := date(2026, time.September, 10)
	newStart := time.Date(2026, time.September, 10, 5, 0, 0, 0, time.UTC)

	var order []string
	repo.On("UpdateTiming", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { order = append(order, "timing") }).Return(nil)
	repo.On("SetMaterializedUntil", mock.Anything, "rule-1", from).
		Run(func(mock.Arguments) { order = append(order, "rewind") }).Return(nil).Once()
	repo.On("DeleteFutureByRule", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { order = append(order, "delete") }).Return(nil)
	repo.On("ReassignToRule", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("GetByID", mock.Anything, "rule-1").Return(rule, nil)
	repo.On("InsertOccurrences", mock.Anything, mock.Anything, mock.Anything).Return(0, nil)
	repo.On("SetMaterializedUntil", mock.Anything, "rule-1", mock.Anything).Return(nil)

	require.NoError(t, svc.Retime(context.Background(), rule, occID, from, newStart, 60, "all"))

	require.Equal(t, []string{"timing", "rewind", "delete"}, order[:3])
}

// Перенос вторничного вхождения на среду меняет в правиле день недели заменой,
// а не добавлением: {2,4} → {3,4}.
func TestRetime_SwapsWeekday(t *testing.T) {
	repo := new(mockRecurrenceRepo)
	svc := service.NewRecurrenceService(repo)
	rule := retimeRule()
	from := date(2026, time.September, 8)                                // вторник
	newStart := time.Date(2026, time.September, 9, 5, 0, 0, 0, time.UTC) // среда, 10:00 Алматы
	newDate := date(2026, time.September, 9)

	repo.On("OccurrenceExists", mock.Anything, "rule-1", newDate, occID).Return(false, nil)
	repo.On("UpdateTiming", mock.Anything, "rule-1", "10:00", 60, []int{3, 4}).Return(nil)
	repo.On("SetMaterializedUntil", mock.Anything, "rule-1", from).Return(nil)
	// cut = min(вт 8, ср 9) = 8; удаляем всё от 8-го, кроме правимого вхождения.
	repo.On("DeleteFutureByRule", mock.Anything, "rule-1", from.AddDate(0, 0, -1), occID).Return(nil)
	repo.On("ReassignToRule", mock.Anything, occID, "rule-1", newDate).Return(nil)
	repo.On("GetByID", mock.Anything, "rule-1").Return(rule, nil)
	repo.On("InsertOccurrences", mock.Anything, "rule-1", mock.Anything).Return(0, nil)
	repo.On("SetMaterializedUntil", mock.Anything, "rule-1", mock.Anything).Return(nil)

	require.NoError(t, svc.Retime(context.Background(), rule, occID, from, newStart, 60, "all"))
	repo.AssertExpectations(t)
}

// Пустой byweekday — поддерживаемое состояние: CreateRule нормализует nil в
// []int{}, а Occurrences в этом случае берёт день недели из starts_on (§2.1
// спеки). swapWeekday обязан материализовать набор тем же способом перед
// подменой, иначе перенос вторника на среду потеряет день вовсе, а не заменит.
func TestRetime_SwapsWeekdayFromEmptyByWeekday(t *testing.T) {
	repo := new(mockRecurrenceRepo)
	svc := service.NewRecurrenceService(repo)
	rule := retimeRule()
	rule.ByWeekday = []int{}                                             // starts_on — вторник 01.09, тот же день недели, что и from
	from := date(2026, time.September, 8)                                // вторник
	newStart := time.Date(2026, time.September, 9, 5, 0, 0, 0, time.UTC) // среда, 10:00 Алматы
	newDate := date(2026, time.September, 9)

	repo.On("OccurrenceExists", mock.Anything, "rule-1", newDate, occID).Return(false, nil)
	// {} материализуется в {isoWeekday(starts_on)} = {2}, вторник заменяется на
	// среду: результат {3}, а не {2,3} и не пустой набор.
	repo.On("UpdateTiming", mock.Anything, "rule-1", "10:00", 60, []int{3}).Return(nil)
	repo.On("SetMaterializedUntil", mock.Anything, "rule-1", from).Return(nil)
	repo.On("DeleteFutureByRule", mock.Anything, "rule-1", from.AddDate(0, 0, -1), occID).Return(nil)
	repo.On("ReassignToRule", mock.Anything, occID, "rule-1", newDate).Return(nil)
	repo.On("GetByID", mock.Anything, "rule-1").Return(rule, nil)
	repo.On("InsertOccurrences", mock.Anything, "rule-1", mock.Anything).Return(0, nil)
	repo.On("SetMaterializedUntil", mock.Anything, "rule-1", mock.Anything).Return(nil)

	require.NoError(t, svc.Retime(context.Background(), rule, occID, from, newStart, 60, "all"))
	repo.AssertExpectations(t)
}

// Перенос на более ранний день недели двигает cut назад: старая среда того же
// правила обязана уйти, иначе ReassignToRule упрётся в уникальный индекс.
func TestRetime_MovingEarlierPullsCutBack(t *testing.T) {
	repo := new(mockRecurrenceRepo)
	svc := service.NewRecurrenceService(repo)
	rule := retimeRule()
	from := date(2026, time.September, 10)                               // четверг
	newStart := time.Date(2026, time.September, 8, 5, 0, 0, 0, time.UTC) // вторник
	cut := date(2026, time.September, 8)

	repo.On("OccurrenceExists", mock.Anything, "rule-1", cut, occID).Return(false, nil)
	repo.On("UpdateTiming", mock.Anything, "rule-1", "10:00", 60, []int{2}).Return(nil)
	repo.On("SetMaterializedUntil", mock.Anything, "rule-1", cut).Return(nil)
	repo.On("DeleteFutureByRule", mock.Anything, "rule-1", cut.AddDate(0, 0, -1), occID).Return(nil)
	repo.On("ReassignToRule", mock.Anything, occID, "rule-1", cut).Return(nil)
	repo.On("GetByID", mock.Anything, "rule-1").Return(rule, nil)
	repo.On("InsertOccurrences", mock.Anything, "rule-1", mock.Anything).Return(0, nil)
	repo.On("SetMaterializedUntil", mock.Anything, "rule-1", mock.Anything).Return(nil)

	require.NoError(t, svc.Retime(context.Background(), rule, occID, from, newStart, 60, "all"))
	repo.AssertExpectations(t)
}

// Целевая дата уже занята вхождением, которое DeleteFutureByRule щадит
// (перенесённым, проведённым или отменённым) — Retime обязан упасть ДО первой
// записи, а не после того, как ReassignToRule упрётся в уникальный индекс.
func TestRetime_ScopeAllRejectsOccupiedDate(t *testing.T) {
	repo := new(mockRecurrenceRepo)
	svc := service.NewRecurrenceService(repo)
	rule := retimeRule()
	from := date(2026, time.September, 8)                                // вторник
	newStart := time.Date(2026, time.September, 9, 5, 0, 0, 0, time.UTC) // среда, 10:00 Алматы
	newDate := date(2026, time.September, 9)

	repo.On("OccurrenceExists", mock.Anything, "rule-1", newDate, occID).Return(true, nil)

	err := svc.Retime(context.Background(), rule, occID, from, newStart, 60, "all")

	require.ErrorIs(t, err, service.ErrConflict)
	repo.AssertNotCalled(t, "UpdateTiming")
	repo.AssertNotCalled(t, "SetMaterializedUntil")
	repo.AssertNotCalled(t, "DeleteFutureByRule")
	repo.AssertNotCalled(t, "ReassignToRule")
}

// «Это и все следующие» разрезает правило: чистим ИСХОДНОЕ правило, а
// материализуем целевое — иначе будущее старой ветки живёт параллельно новой.
func TestRetime_ScopeFollowingSplitsAndPrunesSourceRule(t *testing.T) {
	repo := new(mockRecurrenceRepo)
	svc := service.NewRecurrenceService(repo)
	rule := retimeRule()
	from := date(2026, time.September, 10)
	newStart := time.Date(2026, time.September, 10, 5, 0, 0, 0, time.UTC)
	newRule := models.RecurrenceRule{
		ID: "rule-2", Freq: "weekly", IntervalN: 1, ByWeekday: []int{2, 4},
		TimeLocal: "10:00", TZ: "Asia/Almaty", DurationMinutes: 60,
		StartsOn: from, MaterializedUntil: from,
	}

	repo.On("Split", mock.Anything, "rule-1", from, "10:00", 60, []int{2, 4}).Return(newRule, nil)
	repo.On("DeleteFutureByRule", mock.Anything, "rule-1", from.AddDate(0, 0, -1), occID).Return(nil)
	repo.On("ReassignToRule", mock.Anything, occID, "rule-2", from).Return(nil)
	repo.On("GetByID", mock.Anything, "rule-2").Return(newRule, nil)
	repo.On("InsertOccurrences", mock.Anything, "rule-2", mock.Anything).Return(0, nil)
	repo.On("SetMaterializedUntil", mock.Anything, "rule-2", mock.Anything).Return(nil)

	require.NoError(t, svc.Retime(context.Background(), rule, occID, from, newStart, 60, "following"))

	repo.AssertExpectations(t)
	repo.AssertNotCalled(t, "UpdateTiming")
}

// scope=one до Retime не доезжает: одиночная правка правила не касается.
func TestRetime_RejectsScopeOne(t *testing.T) {
	repo := new(mockRecurrenceRepo)
	svc := service.NewRecurrenceService(repo)

	err := svc.Retime(context.Background(), retimeRule(), occID,
		date(2026, time.September, 10), time.Now(), 60, "one")

	require.ErrorIs(t, err, service.ErrBadRequest)
	repo.AssertNotCalled(t, "UpdateTiming")
	repo.AssertNotCalled(t, "Split")
}
