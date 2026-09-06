package service

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"
	"tutorgo/models"
	"tutorgo/repository"
)

// RecurrenceHorizon — насколько вперёд держим материализованные вхождения.
// «Спортзал каждый понедельник» не имеет конца, а строки в БД имеют, поэтому
// горизонт скользящий: ночная джоба дотягивает его для активных правил.
const RecurrenceHorizon = 6 * 30 * 24 * time.Hour

type RecurrenceService interface {
	// Materialize добивает вхождения правила до horizon и двигает границу.
	Materialize(ctx context.Context, ruleID string, horizon time.Time) (int, error)
	// ExtendAll — то же для всех правил, чей горизонт короче: точка входа джобы.
	ExtendAll(ctx context.Context, horizon time.Time) (int, error)
	// CreateRule собирает правило из первого вхождения: время и длительность
	// берутся оттуда, а не спрашиваются вторично.
	CreateRule(ctx context.Context, in models.RecurrenceInput, firstAt time.Time, durationMinutes int, tutorID string) (models.RecurrenceRule, error)
	DeleteRule(ctx context.Context, ruleID string) error
	GetRule(ctx context.Context, ruleID string) (models.RecurrenceRule, error)
	CloseRule(ctx context.Context, ruleID string, at time.Time) error
	// PruneFuture убирает будущие вхождения правила: удаление «это и все
	// следующие» и «все» ходят через него.
	PruneFuture(ctx context.Context, ruleID string, after time.Time) error
}

// Область правки вхождения серии.
const (
	scopeOne       = "one"
	scopeFollowing = "following"
	scopeAll       = "all"
)

// localTimeIn — «17:00» в зоне правила: правило хранит стенные часы, а не UTC.
func localTimeIn(at time.Time, tz string) (string, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return "", fmt.Errorf("timezone %q: %w", tz, ErrBadRequest)
	}
	return at.In(loc).Format("15:04"), nil
}

type recurrenceService struct {
	repo repository.RecurrenceRepository
	log  *slog.Logger
}

func NewRecurrenceService(repo repository.RecurrenceRepository) RecurrenceService {
	return &recurrenceService{repo: repo, log: slog.Default()}
}

func (s *recurrenceService) CreateRule(ctx context.Context, in models.RecurrenceInput, firstAt time.Time, durationMinutes int, tutorID string) (models.RecurrenceRule, error) {
	loc, err := time.LoadLocation(in.TZ)
	if err != nil {
		return models.RecurrenceRule{}, fmt.Errorf("timezone %q: %w", in.TZ, ErrBadRequest)
	}
	local := firstAt.In(loc)
	startsOn := dayOf(local)

	rule := models.RecurrenceRule{
		TutorID:         tutorID,
		Freq:            in.Freq,
		IntervalN:       max(in.IntervalN, 1),
		ByWeekday:       in.ByWeekday,
		TimeLocal:       local.Format("15:04"),
		TZ:              in.TZ,
		DurationMinutes: durationMinutes,
		StartsOn:        startsOn,
		EndsOn:          in.EndsOn,
		MaxCount:        in.MaxCount,
		// Первое вхождение создаёт вызывающий, поэтому граница — день старта:
		// материализация добирает всё, что дальше.
		MaterializedUntil: startsOn,
	}
	if rule.ByWeekday == nil {
		rule.ByWeekday = []int{}
	}
	return s.repo.Create(ctx, rule)
}

func (s *recurrenceService) DeleteRule(ctx context.Context, ruleID string) error {
	return s.repo.Delete(ctx, ruleID)
}

func (s *recurrenceService) GetRule(ctx context.Context, ruleID string) (models.RecurrenceRule, error) {
	rule, err := s.repo.GetByID(ctx, ruleID)
	if err != nil {
		return models.RecurrenceRule{}, fmt.Errorf("rule: %w", ErrNotFound)
	}
	return rule, nil
}

func (s *recurrenceService) PruneFuture(ctx context.Context, ruleID string, after time.Time) error {
	return s.repo.DeleteFutureByRule(ctx, ruleID, after, "")
}

// CloseRule обрывает серию на дате `at`: всё, что дальше, серии больше не
// принадлежит. Само вхождение отменяет вызывающий — здесь только правило.
func (s *recurrenceService) CloseRule(ctx context.Context, ruleID string, at time.Time) error {
	return s.repo.SetEndsOn(ctx, ruleID, at.AddDate(0, 0, -1))
}

func (s *recurrenceService) Materialize(ctx context.Context, ruleID string, horizon time.Time) (int, error) {
	rule, err := s.repo.GetByID(ctx, ruleID)
	if err != nil {
		return 0, fmt.Errorf("rule: %w", ErrNotFound)
	}
	if !rule.MaterializedUntil.Before(horizon) {
		return 0, nil // горизонт уже дальше — двигать назад нечего
	}

	// Считаем от уже материализованной границы; повторно попавшие даты отсечёт
	// уникальный индекс (rule_id, occurrence_date), поэтому граница включительная.
	starts, err := Occurrences(rule, rule.MaterializedUntil, horizon)
	if err != nil {
		return 0, err
	}

	var n int
	if len(starts) > 0 {
		if n, err = s.repo.InsertOccurrences(ctx, rule.ID, starts); err != nil {
			return 0, err
		}
	}
	// Границу двигаем и когда вхождений нет: закончившееся правило иначе будет
	// вечно возвращаться в выборку джобы.
	if err := s.repo.SetMaterializedUntil(ctx, rule.ID, horizon); err != nil {
		return n, err
	}
	return n, nil
}

func (s *recurrenceService) ExtendAll(ctx context.Context, horizon time.Time) (int, error) {
	rules, err := s.repo.DueForMaterialization(ctx, horizon)
	if err != nil {
		return 0, err
	}

	var total int
	for _, rule := range rules {
		// Битое правило (например, неизвестная зона) не должно ронять всю
		// ночную догрузку — логируем и идём дальше.
		n, err := s.Materialize(ctx, rule.ID, horizon)
		if err != nil {
			s.log.Error("materialize rule", slog.String("rule_id", rule.ID), slog.String("error", err.Error()))
			continue
		}
		total += n
	}
	return total, nil
}

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
