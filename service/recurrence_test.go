package service_test

import (
	"testing"
	"time"
	"tutorgo/models"
	"tutorgo/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func baseRule() models.RecurrenceRule {
	return models.RecurrenceRule{
		Freq:            "weekly",
		IntervalN:       1,
		TimeLocal:       "17:00",
		TZ:              "Asia/Almaty",
		DurationMinutes: 60,
		StartsOn:        date(2026, time.September, 1), // вторник
	}
}

// В окно попадают только вхождения [from, to), время — локальные 17:00.
func TestOccurrences_WeeklyByWeekday(t *testing.T) {
	rule := baseRule()
	rule.ByWeekday = []int{2, 4} // вторник и четверг

	got, err := service.Occurrences(rule, date(2026, time.September, 1), date(2026, time.September, 15))
	require.NoError(t, err)

	require.Len(t, got, 4)
	almaty, _ := time.LoadLocation("Asia/Almaty")
	assert.Equal(t, time.Date(2026, 9, 1, 17, 0, 0, 0, almaty), got[0].In(almaty))
	assert.Equal(t, time.Date(2026, 9, 3, 17, 0, 0, 0, almaty), got[1].In(almaty))
	assert.Equal(t, time.Date(2026, 9, 8, 17, 0, 0, 0, almaty), got[2].In(almaty))
	assert.Equal(t, time.Date(2026, 9, 10, 17, 0, 0, 0, almaty), got[3].In(almaty))
}

// Пустой byweekday — день недели берётся из starts_on.
func TestOccurrences_WeeklyWithoutWeekdays(t *testing.T) {
	rule := baseRule()

	got, err := service.Occurrences(rule, date(2026, time.September, 1), date(2026, time.September, 22))
	require.NoError(t, err)

	require.Len(t, got, 3) // 1, 8, 15 сентября; 22-е — уже за границей окна
	for _, at := range got {
		assert.Equal(t, time.Tuesday, at.In(time.UTC).Weekday())
	}
}

func TestOccurrences_EveryTwoWeeks(t *testing.T) {
	rule := baseRule()
	rule.IntervalN = 2

	got, err := service.Occurrences(rule, date(2026, time.September, 1), date(2026, time.October, 1))
	require.NoError(t, err)

	// 1, 15, 29 сентября — вторники через неделю.
	require.Len(t, got, 3)
	almaty, _ := time.LoadLocation("Asia/Almaty")
	assert.Equal(t, 1, got[0].In(almaty).Day())
	assert.Equal(t, 15, got[1].In(almaty).Day())
	assert.Equal(t, 29, got[2].In(almaty).Day())
}

// Главная причина хранить tz, а не смещение: по обе стороны перевода часов
// урок остаётся в 17:00 по стенным часам, хотя UTC-время сдвигается.
func TestOccurrences_KeepsLocalTimeAcrossDST(t *testing.T) {
	rule := baseRule()
	rule.TZ = "Europe/Berlin"
	rule.StartsOn = date(2026, time.October, 20) // вторник до перевода 25 октября

	got, err := service.Occurrences(rule, date(2026, time.October, 20), date(2026, time.November, 10))
	require.NoError(t, err)

	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	require.Len(t, got, 3)
	for _, at := range got {
		local := at.In(berlin)
		assert.Equal(t, 17, local.Hour(), "локальное время должно оставаться 17:00")
	}
	// А UTC-время обязано сдвинуться на час: 20 октября ещё CEST, 3 ноября — CET.
	assert.NotEqual(t, got[0].UTC().Hour(), got[2].UTC().Hour())
}

func TestOccurrences_MonthlySkipsMissingDay(t *testing.T) {
	rule := baseRule()
	rule.Freq = "monthly"
	rule.StartsOn = date(2027, time.January, 31)

	got, err := service.Occurrences(rule, date(2027, time.January, 1), date(2027, time.June, 1))
	require.NoError(t, err)

	// 31-го нет ни в феврале, ни в апреле — эти месяцы пропускаются целиком.
	require.Len(t, got, 3)
	almaty, _ := time.LoadLocation("Asia/Almaty")
	assert.Equal(t, time.January, got[0].In(almaty).Month())
	assert.Equal(t, time.March, got[1].In(almaty).Month())
	assert.Equal(t, time.May, got[2].In(almaty).Month())
}

func TestOccurrences_Daily(t *testing.T) {
	rule := baseRule()
	rule.Freq = "daily"
	rule.IntervalN = 3

	got, err := service.Occurrences(rule, date(2026, time.September, 1), date(2026, time.September, 11))
	require.NoError(t, err)

	require.Len(t, got, 4) // 1, 4, 7, 10 сентября
}

func TestOccurrences_StopsOnEndsOn(t *testing.T) {
	rule := baseRule()
	ends := date(2026, time.September, 16)
	rule.EndsOn = &ends

	got, err := service.Occurrences(rule, date(2026, time.September, 1), date(2026, time.December, 1))
	require.NoError(t, err)

	require.Len(t, got, 3) // 1, 8, 15 сентября; 22-е уже за ends_on
}

// max_count считается от starts_on, а не от начала окна: иначе догрузка
// следующего куска горизонта выдала бы ещё столько же вхождений.
func TestOccurrences_MaxCountCountsFromStart(t *testing.T) {
	rule := baseRule()
	max := 3
	rule.MaxCount = &max

	got, err := service.Occurrences(rule, date(2026, time.September, 8), date(2026, time.December, 1))
	require.NoError(t, err)

	// Всего правило даёт три вхождения: 1, 8, 15 сентября. Окно начинается
	// восьмым, поэтому вернутся два — но не «ещё три, считая от окна».
	require.Len(t, got, 2)
}

func TestOccurrences_UnknownTimezone(t *testing.T) {
	rule := baseRule()
	rule.TZ = "Mars/Olympus"

	_, err := service.Occurrences(rule, date(2026, time.September, 1), date(2026, time.October, 1))

	assert.Error(t, err)
}
