package service

import (
	"fmt"
	"slices"
	"time"
	"tutorgo/models"
)

// maxScannedDates — предохранитель от бесконечного цикла, если правило и окно
// заданы так, что подходящих дат не находится (например, monthly 31-го и узкое
// окно): 20 лет календарных дат заведомо перекрывают любой разумный горизонт.
const maxScannedDates = 366 * 20

// Occurrences возвращает моменты начала вхождений правила в полуинтервале [from, to).
//
// Время считается как локальные стенные часы rule.TimeLocal в зоне rule.TZ и
// пересобирается для каждой календарной даты отдельно — это и есть защита от
// перевода часов: складывать длительности в UTC нельзя, иначе после перехода
// урок уедет на час.
func Occurrences(rule models.RecurrenceRule, from, to time.Time) ([]time.Time, error) {
	loc, err := time.LoadLocation(rule.TZ)
	if err != nil {
		return nil, fmt.Errorf("timezone %q: %w", rule.TZ, ErrBadRequest)
	}
	hh, mm, err := parseTimeLocal(rule.TimeLocal)
	if err != nil {
		return nil, err
	}
	interval := max(rule.IntervalN, 1)

	// Идём от starts_on, а не от from: max_count считается от начала правила,
	// иначе догрузка следующего куска горизонта выдала бы ещё столько же.
	start := dayOf(rule.StartsOn)
	var (
		result []time.Time
		count  int
	)
	for day, scanned := start, 0; scanned < maxScannedDates; scanned++ {
		if rule.EndsOn != nil && day.After(dayOf(*rule.EndsOn)) {
			break
		}
		if rule.MaxCount != nil && count >= *rule.MaxCount {
			break
		}

		next, matched := nextDate(rule, day, start, interval)
		if matched {
			at := time.Date(day.Year(), day.Month(), day.Day(), hh, mm, 0, 0, loc)
			if !at.Before(to) {
				break
			}
			count++
			if !at.Before(from) {
				result = append(result, at)
			}
		}
		day = next
	}
	return result, nil
}

// nextDate отвечает на два вопроса разом: подходит ли дата под правило и какую
// дату проверять следующей. Разнесённые по функциям, они разъезжались бы:
// у monthly шаг зависит от того, существует ли нужное число в месяце.
func nextDate(rule models.RecurrenceRule, day, start time.Time, interval int) (time.Time, bool) {
	switch rule.Freq {
	case "daily":
		return day.AddDate(0, 0, interval), true

	case "monthly":
		// Тот же день месяца, что у starts_on; месяца без такого числа просто нет
		// в серии (31-е в феврале), поэтому шагаем по месяцам от starts_on.
		months := (day.Year()-start.Year())*12 + int(day.Month()-start.Month())
		candidate := start.AddDate(0, months+interval, 0)
		// AddDate нормализует 31 февраля в 3 марта — такие месяцы пропускаем.
		for candidate.Day() != start.Day() {
			months += interval
			candidate = start.AddDate(0, months+interval, 0)
		}
		return candidate, day.Day() == start.Day()

	default: // weekly
		days := rule.ByWeekday
		if len(days) == 0 {
			days = []int{isoWeekday(start)}
		}
		// Отсчёт недель — от недели starts_on, чтобы «каждые 2 недели» не
		// зависело от того, с какого дня недели правило начали.
		weeks := int(mondayOf(day).Sub(mondayOf(start)).Hours() / 24 / 7)
		matched := weeks%interval == 0 && slices.Contains(days, isoWeekday(day))
		return day.AddDate(0, 0, 1), matched
	}
}

func parseTimeLocal(s string) (hour, minute int, err error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		// TIME из Postgres приходит как «17:00:00».
		t, err = time.Parse("15:04:05", s)
		if err != nil {
			return 0, 0, fmt.Errorf("time_local %q: %w", s, ErrBadRequest)
		}
	}
	return t.Hour(), t.Minute(), nil
}

func dayOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func mondayOf(t time.Time) time.Time {
	return dayOf(t).AddDate(0, 0, -(isoWeekday(t) - 1))
}

// isoWeekday: 1=Пн … 7=Вс (time.Weekday считает воскресенье нулём).
func isoWeekday(t time.Time) int {
	return (int(t.Weekday())+6)%7 + 1
}
