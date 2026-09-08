package service_test

import (
	"testing"
	"time"
	"tutorgo/models"
	"tutorgo/service"

	"github.com/stretchr/testify/require"
)

// Правило-образец: еженедельно по вторникам и четвергам, 17:00 в Алматы.
func movableRule() models.RecurrenceRule {
	return models.RecurrenceRule{
		ID: "rule-1", Freq: "weekly", IntervalN: 1, ByWeekday: []int{2, 4},
		TimeLocal: "17:00", TZ: "Asia/Almaty", DurationMinutes: 60,
		StartsOn: date(2026, time.September, 1),
	}
}

// Та же дата, другое время — самый частый случай: серию двигают по часам.
func TestNormalizeSeriesStart_TimeOnly(t *testing.T) {
	from := date(2026, time.September, 10) // четверг
	newStart := time.Date(2026, time.September, 10, 5, 0, 0, 0, time.UTC) // 10:00 Алматы

	got, err := service.NormalizeSeriesStart(movableRule(), from, newStart)

	require.NoError(t, err)
	loc, _ := time.LoadLocation("Asia/Almaty")
	require.Equal(t, "2026-09-10 10:00", got.In(loc).Format("2006-01-02 15:04"))
}

// Другая неделя, тот же день недели — неделя игнорируется: серию задают день
// недели и время, а не выбранная в форме дата.
func TestNormalizeSeriesStart_IgnoresWeek(t *testing.T) {
	from := date(2026, time.September, 10) // четверг
	newStart := time.Date(2026, time.September, 17, 5, 0, 0, 0, time.UTC) // четверг следующей недели

	got, err := service.NormalizeSeriesStart(movableRule(), from, newStart)

	require.NoError(t, err)
	loc, _ := time.LoadLocation("Asia/Almaty")
	require.Equal(t, "2026-09-10 10:00", got.In(loc).Format("2006-01-02 15:04"))
}

// Другой день недели — переезд внутри недели переносимого вхождения:
// среда 23-го читается как среда 9-го.
func TestNormalizeSeriesStart_MovesWeekdayWithinOwnWeek(t *testing.T) {
	from := date(2026, time.September, 10) // четверг
	newStart := time.Date(2026, time.September, 23, 5, 0, 0, 0, time.UTC) // среда через две недели

	got, err := service.NormalizeSeriesStart(movableRule(), from, newStart)

	require.NoError(t, err)
	loc, _ := time.LoadLocation("Asia/Almaty")
	require.Equal(t, "2026-09-09 10:00", got.In(loc).Format("2006-01-02 15:04"))
}

// У daily и monthly дата выводится из starts_on, переносить её этой операцией
// нельзя — честный 400 вместо тихой порчи правила.
func TestNormalizeSeriesStart_RejectsDateMoveForMonthly(t *testing.T) {
	rule := movableRule()
	rule.Freq = "monthly"
	from := date(2026, time.September, 10)
	newStart := time.Date(2026, time.September, 11, 5, 0, 0, 0, time.UTC)

	_, err := service.NormalizeSeriesStart(rule, from, newStart)

	require.ErrorIs(t, err, service.ErrBadRequest)
}

// Битая зона — тоже 400, а не паника внутри time.LoadLocation.
func TestNormalizeSeriesStart_RejectsBadTimezone(t *testing.T) {
	rule := movableRule()
	rule.TZ = "Nowhere/Nothing"

	_, err := service.NormalizeSeriesStart(rule, date(2026, time.September, 10), time.Now())

	require.ErrorIs(t, err, service.ErrBadRequest)
}
