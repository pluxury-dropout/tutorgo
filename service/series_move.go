package service

import (
	"fmt"
	"slices"
	"time"
	"tutorgo/models"
)

// NormalizeSeriesStart приводит время запроса к дате, которую примет правило.
//
// При scope != one серию задают день недели и время, а выбранная в форме неделя
// роли не играет: «чт 10.09 → чт 17.09» — это чистая смена времени, а
// «чт 10.09 → ср 23.09» — переезд серии на среду недели переносимого вхождения,
// то есть на 09.09. Без нормализации строка вхождения и occurrence_date
// разъедутся: правило пересчитается по одной дате, а урок встанет на другую.
//
// Для freq != "weekly" смена даты отклоняется: у daily она бессмысленна, у
// monthly день месяца выводится из starts_on и требует другой операции.
func NormalizeSeriesStart(rule models.RecurrenceRule, from, newStart time.Time) (time.Time, error) {
	loc, err := time.LoadLocation(rule.TZ)
	if err != nil {
		return time.Time{}, fmt.Errorf("timezone %q: %w", rule.TZ, ErrBadRequest)
	}
	local := newStart.In(loc)
	target := dayOf(local)
	origin := dayOf(from)

	day := origin
	if rule.Freq == "weekly" {
		// День недели берём у выбранной даты, неделю — у переносимого вхождения.
		day = origin.AddDate(0, 0, isoWeekday(target)-isoWeekday(origin))
	} else if !target.Equal(origin) {
		return time.Time{}, fmt.Errorf("серия %q не поддерживает перенос по датам: %w", rule.Freq, ErrBadRequest)
	}

	return time.Date(day.Year(), day.Month(), day.Day(), local.Hour(), local.Minute(), 0, 0, loc), nil
}

// swapWeekday меняет в наборе день переносимого вхождения на новый: правило
// {вт,чт} при переносе вторника на среду становится {ср,чт}, а не {вт,ср,чт}
// (серия стала бы трёхдневной) и не {ср} (четверг потерялся бы).
//
// Пустой набор материализуется из starts_on — Occurrences трактует его именно так.
func swapWeekday(rule models.RecurrenceRule, oldDay, newDay int) []int {
	days := rule.ByWeekday
	if len(days) == 0 {
		days = []int{isoWeekday(rule.StartsOn)}
	}
	out := make([]int, 0, len(days))
	for _, d := range days {
		if d != oldDay {
			out = append(out, d)
		}
	}
	// Новый день уже в наборе — серия схлопывается в однодневную. Это ровно то,
	// о чём попросил пользователь; отдельной обработки не нужно.
	if !slices.Contains(out, newDay) {
		out = append(out, newDay)
	}
	slices.Sort(out)
	return out
}
