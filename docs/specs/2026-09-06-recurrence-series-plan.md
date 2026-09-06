# Перенос серий (Retime) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** свести логику переноса серии в один метод `RecurrenceService.Retime`, починить восемь дефектов вокруг `recurrence_rules` и оживить серийный интерфейс.

**Architecture:** вся арифметика правила (пересчёт `byweekday`, `Split`/`UpdateTiming`, откат `materialized_until`, порядок «почистить → переставить → материализовать») переезжает в один метод `RecurrenceService.Retime`, который обслуживает и уроки, и события. Методы «по `rule_id`» (`DeleteFutureByRule`, `ReassignToRule`) переезжают в `RecurrenceRepository` и работают с обеими таблицами — так же, как уже устроен `InsertOccurrences`. `service/event.go:applyToSeries` удаляется целиком.

**Tech Stack:** Go 1.2x, Gin, pgx/v5, testify/mock, goose; фронт — Next.js App Router, React Query, shadcn/ui.

**Spec:** `docs/specs/2026-09-06-recurrence-series-design.md`

## Global Constraints

- **Правки файлов — только через Edit/Write**, не через `sed`/`python`/heredoc в Bash (`CLAUDE.md`). Чтение через `cat`/`sed -n`/`grep` — нормально.
- **Рассинхрон интерфейса и моков ломает компиляцию тестов** — главный footgun проекта. Меняешь сигнатуру в `repository/` или `service/` — сразу правь `service/*_test.go` и `handlers/mocks_test.go`.
- Тесты сервисного слоя живут в `package service_test` (внешний пакет): **неэкспортированные функции оттуда не видны**, покрывать их надо через экспортированную поверхность.
- Порядок коммитов обязателен: Задачи 1–4 (закрывают разрушение данных) идут **до** Задачи 5, которая открывает пользователю ту же кнопку.
- Интеграционные тесты: тег сборки `//go:build integration`, база через **`TEST_DB_URL`** (отдельная от `DB_URL` — `.env` смотрит в прод-Supabase), пропуск через `t.Skip` при пустой переменной. Образец — `repository/whiteboard_elements_integration_test.go:33` (`testPool`).
- Комментарии в коде — по-русски, объясняют «почему», а не «что»: следуй стилю соседних файлов.
- `make test` должен быть зелёным после каждой задачи.

## Порядок и параллельность

Задачи 1 → 7 строго последовательны: каждая опирается на сигнатуры предыдущей.

**Задача 8 (фронт) не пересекается с бэкендом ни одним файлом и запускается параллельно с Задачами 1–7.** Это единственная честная точка параллелизма в плане: всё остальное — один и тот же `repository/lesson.go`, `service/lesson.go`, `router/router.go`.

| задача | коммит спеки | закрывает дефекты |
|---|---|---|
| 1–4 | 1 | A (`scope=all` сносит будущее), B (день недели), F (крэш-безопасность), G (дублирование), H (кеш) |
| 5 | 2 | C (`rule_id` не селектится) |
| 6–7 | 3 | D (архивация), остаток бага 3 (сироты) |
| 8 | 4 | E (попап урока) |

---

### Task 1: `NormalizeSeriesStart` и `swapWeekday`

Чистые функции без БД: вся арифметика дат и дней недели, которую дальше зовут и `Retime`, и вызывающие сервисы.

**Files:**
- Create: `service/series_move.go`
- Test: `service/series_move_test.go`

**Interfaces:**
- Consumes: `models.RecurrenceRule`; из `service/recurrence.go` — приватные `dayOf`, `isoWeekday`, переменную `ErrBadRequest`.
- Produces:
  - `func NormalizeSeriesStart(rule models.RecurrenceRule, from, newStart time.Time) (time.Time, error)`
  - `func swapWeekday(rule models.RecurrenceRule, oldDay, newDay int) []int` (приватная, покрывается через Задачу 3)

- [ ] **Step 1: Написать падающий тест**

Создать `service/series_move_test.go`:

```go
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
```

- [ ] **Step 2: Запустить тест и убедиться, что он падает**

Run: `go test ./service/ -run TestNormalizeSeriesStart -v`
Expected: FAIL — `undefined: service.NormalizeSeriesStart`

- [ ] **Step 3: Написать реализацию**

Создать `service/series_move.go`:

```go
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
```

- [ ] **Step 4: Запустить тесты и убедиться, что они проходят**

Run: `go test ./service/ -run TestNormalizeSeriesStart -v`
Expected: PASS, пять тестов

Run: `go test ./...`
Expected: PASS (`swapWeekday` пока не вызывается — Go не ругается на неиспользуемые функции, только на переменные)

- [ ] **Step 5: Коммит**

```bash
git add service/series_move.go service/series_move_test.go
git commit -m "feat(recurrence): нормализация даты и подмена дня недели для переноса серии"
```

---

### Task 2: поверхность репозиториев

Правило владеет вхождениями в обеих таблицах, поэтому методы «по `rule_id`» съезжаются в `RecurrenceRepository` — так же, как уже устроен `InsertOccurrences`, где один INSERT попадает, а второй не находит шаблона.

**Files:**
- Modify: `repository/recurrence.go:11-21` (интерфейс), `:92` (`Split`), `:117` (`UpdateTiming`), конец файла (два новых метода)
- Modify: `repository/lesson.go:22-23` (интерфейс), `:~145` (`ReassignToRule`), `:~176` (`DeleteFutureByRule`) — оба удаляются
- Modify: `repository/event.go:25-26` (интерфейс), `:~84` (`ReassignToRule`), `:~90` (`DeleteFutureByRule`) — оба удаляются
- Modify: `service/recurrence.go:20-33` (интерфейс `RecurrenceService`), `:101` (`SplitRule`), `:105` (`UpdateRuleTiming`)
- Modify: `service/lesson.go:256-299` (`updateSeries`), `:301-335` (`Delete`)
- Modify: `service/event.go:73-106` (`Update`), `:112-141` (`applyToSeries` удаляется), `:143-` (`Delete`)
- Modify: `service/scope_test.go:19-38` (моки), `service/event_test.go:48-55` (моки)

**Interfaces:**
- Produces (`repository.RecurrenceRepository`):
  - `Split(ctx context.Context, ruleID string, at time.Time, timeLocal string, duration int, byweekday []int) (models.RecurrenceRule, error)`
  - `UpdateTiming(ctx context.Context, ruleID, timeLocal string, duration int, byweekday []int) error`
  - `DeleteFutureByRule(ctx context.Context, ruleID string, after time.Time, exceptID string) error`
  - `ReassignToRule(ctx context.Context, occurrenceID, ruleID string, occurrenceDate time.Time) error`
- Produces (`service.RecurrenceService`): `PruneFuture(ctx context.Context, ruleID string, after time.Time) error`
- Removes: `LessonRepository.DeleteFutureByRule`, `LessonRepository.ReassignToRule`, `EventRepository.DeleteFutureByRule`, `EventRepository.ReassignToRule`, `RecurrenceService.SplitRule`, `RecurrenceService.UpdateRuleTiming`

- [ ] **Step 1: Расширить `Split` и `UpdateTiming` в `repository/recurrence.go`**

В интерфейсе (`:18-19`) заменить две строки:

```go
	Split(ctx context.Context, ruleID string, at time.Time, timeLocal string, duration int, byweekday []int) (models.RecurrenceRule, error)
	UpdateTiming(ctx context.Context, ruleID, timeLocal string, duration int, byweekday []int) error
	DeleteFutureByRule(ctx context.Context, ruleID string, after time.Time, exceptID string) error
	ReassignToRule(ctx context.Context, occurrenceID, ruleID string, occurrenceDate time.Time) error
```

Реализации:

```go
func (r *recurrenceRepository) Split(ctx context.Context, ruleID string, at time.Time, timeLocal string, duration int, byweekday []int) (models.RecurrenceRule, error) {
	if _, err := r.pool.Exec(ctx,
		`UPDATE recurrence_rules SET ends_on = $2::date - 1 WHERE id = $1`, ruleID, at,
	); err != nil {
		return models.RecurrenceRule{}, err
	}

	return scanRule(r.pool.QueryRow(ctx,
		`INSERT INTO recurrence_rules
		     (tutor_id, freq, interval_n, byweekday, time_local, tz, duration_minutes,
		      starts_on, ends_on, max_count, materialized_until)
		 SELECT tutor_id, freq, interval_n, $5::smallint[], $3::time, tz, $4,
		        $2::date, NULL, max_count, $2::date
		 FROM recurrence_rules WHERE id = $1
		 RETURNING `+ruleCols,
		ruleID, at, timeLocal, duration, byweekday,
	))
}

func (r *recurrenceRepository) UpdateTiming(ctx context.Context, ruleID, timeLocal string, duration int, byweekday []int) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE recurrence_rules
		 SET time_local = $2::time, duration_minutes = $3, byweekday = $4::smallint[]
		 WHERE id = $1`,
		ruleID, timeLocal, duration, byweekday)
	return err
}
```

- [ ] **Step 2: Добавить `DeleteFutureByRule` и `ReassignToRule` в `repository/recurrence.go`**

Дописать в конец файла:

```go
// DeleteFutureByRule убирает вхождения правила начиная с `after` + 1 день, из
// обеих таблиц сразу: правило владеет либо уроками, либо событиями, и второй
// запрос просто ничего не находит — тот же приём, что в InsertOccurrences.
//
// Не трогает вручную перенесённые (is_override) и уже отменённые: и те, и
// другие — осознанные решения пользователя. exceptID щадит вхождение, которое
// вызывающий переставляет сам; пустая строка означает «щадить нечего».
func (r *recurrenceRepository) DeleteFutureByRule(ctx context.Context, ruleID string, after time.Time, exceptID string) error {
	if _, err := r.pool.Exec(ctx,
		`DELETE FROM lessons
		 WHERE rule_id = $1::uuid AND occurrence_date > $2::date
		   AND is_override = FALSE AND status = 'scheduled'
		   AND ($3 = '' OR id <> $3::uuid)`, ruleID, after, exceptID,
	); err != nil {
		return err
	}
	_, err := r.pool.Exec(ctx,
		`DELETE FROM events
		 WHERE rule_id = $1::uuid AND occurrence_date > $2::date
		   AND is_override = FALSE AND NOT cancelled
		   AND ($3 = '' OR id <> $3::uuid)`, ruleID, after, exceptID)
	return err
}

// ReassignToRule переводит вхождение в другое правило и на другую дату.
// is_override снимается: правку сделали через саму серию, а не вопреки ей.
func (r *recurrenceRepository) ReassignToRule(ctx context.Context, occurrenceID, ruleID string, occurrenceDate time.Time) error {
	if _, err := r.pool.Exec(ctx,
		`UPDATE lessons SET rule_id = $2::uuid, occurrence_date = $3::date, is_override = FALSE
		 WHERE id = $1::uuid`, occurrenceID, ruleID, occurrenceDate,
	); err != nil {
		return err
	}
	_, err := r.pool.Exec(ctx,
		`UPDATE events SET rule_id = $2::uuid, occurrence_date = $3::date, is_override = FALSE
		 WHERE id = $1::uuid`, occurrenceID, ruleID, occurrenceDate)
	return err
}
```

- [ ] **Step 3: Удалить переехавшие методы из `repository/lesson.go` и `repository/event.go`**

Из интерфейса `LessonRepository` убрать строки `ReassignToRule` и `DeleteFutureByRule`, из тела файла — обе функции с их докблоками. То же в `EventRepository` и `repository/event.go`.

- [ ] **Step 4: Добавить `PruneFuture` в `service/recurrence.go`, убрать `SplitRule` и `UpdateRuleTiming`**

В интерфейсе `RecurrenceService` удалить строки `SplitRule` и `UpdateRuleTiming` (после Задачи 3 их никто не зовёт), добавить:

```go
	// PruneFuture убирает будущие вхождения правила: удаление «это и все
	// следующие» и «все» ходят через него.
	PruneFuture(ctx context.Context, ruleID string, after time.Time) error
```

Тела `SplitRule` (`:101`) и `UpdateRuleTiming` (`:105`) удалить, добавить:

```go
func (s *recurrenceService) PruneFuture(ctx context.Context, ruleID string, after time.Time) error {
	return s.repo.DeleteFutureByRule(ctx, ruleID, after, "")
}
```

- [ ] **Step 5: Перевести места удаления серий на `PruneFuture`**

`service/lesson.go`, метод `Delete`: заменить оба `s.repo.DeleteFutureByRule(...)` на `s.recurrence.PruneFuture(...)` с теми же аргументами (`*lesson.RuleID, *lesson.OccurrenceDate` и `*lesson.RuleID, time.Time{}`).

`service/event.go`, метод `Delete`, ветка `scopeFollowing`: заменить `s.repo.DeleteFutureByRule(ctx, *current.RuleID, *current.OccurrenceDate)` на `s.recurrence.PruneFuture(ctx, *current.RuleID, *current.OccurrenceDate)`.

Временно в `service/lesson.go:updateSeries` и `service/event.go:applyToSeries` заменить вызовы `s.repo.DeleteFutureByRule`, `s.repo.ReassignToRule`, `s.recurrence.SplitRule`, `s.recurrence.UpdateRuleTiming` на компилируемые заглушки — обе функции целиком перепишет Задача 3. Проще всего: временно оставить в них только `return models.Lesson{}, fmt.Errorf("not implemented")` / `return fmt.Errorf("not implemented")` и снять из них тесты в Шаге 6.

- [ ] **Step 6: Синхронизировать моки**

`service/scope_test.go` — четыре мока правятся, два удаляются:

```go
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
```

Удалить `mockLessonRepo.ReassignToRule`, `mockLessonRepo.DeleteFutureByRule` (`scope_test.go:19-24`) и `mockEventRepo.ReassignToRule`, `mockEventRepo.DeleteFutureByRule` (`event_test.go:48-55`).

Тесты `TestLessonUpdate_ScopeFollowing` и `TestLessonUpdate_ScopeAll` в `scope_test.go` временно пометить `t.Skip("переписывается в Задаче 3")` — их полностью заменит Задача 3.

- [ ] **Step 7: Проверить сборку и тесты**

Run: `go build ./... && go test ./...`
Expected: PASS. Если падает компиляция тестов — ищи несинхронизированный мок, это ровно тот footgun.

- [ ] **Step 8: Коммит**

```bash
git add repository/ service/
git commit -m "refactor(recurrence): методы по rule_id переезжают в RecurrenceRepository"
```

---

### Task 3: `RecurrenceService.Retime` и переписанные вызывающие

Ядро. Здесь закрываются дефекты A, B, F, G и H разом.

**Files:**
- Modify: `service/recurrence.go` (интерфейс + новый метод)
- Modify: `service/lesson.go:256-299` (`updateSeries`)
- Modify: `service/event.go:73-141` (`Update`, `applyToSeries` удаляется)
- Test: `service/scope_test.go` (переписать два теста), `service/retime_test.go` (создать)

**Interfaces:**
- Consumes: `NormalizeSeriesStart`, `swapWeekday` (Задача 1); репозиторные сигнатуры (Задача 2)
- Produces: `Retime(ctx context.Context, rule models.RecurrenceRule, occurrenceID string, from, newStart time.Time, duration int, scope string) error`

- [ ] **Step 1: Написать падающие тесты**

Создать `service/retime_test.go`:

```go
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

// Правило: еженедельно вт+чт, 17:00 Алматы, горизонт уже дотянут до марта.
func retimeRule() models.RecurrenceRule {
	return models.RecurrenceRule{
		ID: "rule-1", TutorID: tutorID, Freq: "weekly", IntervalN: 1,
		ByWeekday: []int{2, 4}, TimeLocal: "17:00", TZ: "Asia/Almaty",
		DurationMinutes: 60,
		StartsOn:        date(2026, time.September, 1),
		// Ровно та граница, из-за которой Materialize раньше выходил сразу и
		// оставлял окно между «сегодня» и горизонтом пустым.
		MaterializedUntil: date(2027, time.March, 5),
	}
}

// «Изменить все» откатывает границу материализации на дату разреза — иначе
// Materialize выйдет по !MaterializedUntil.Before(horizon) и будущие вхождения,
// только что удалённые, никто не пересоздаст.
func TestRetime_ScopeAllRewindsWatermark(t *testing.T) {
	repo := new(mockRecurrenceRepo)
	svc := service.NewRecurrenceService(repo)
	rule := retimeRule()
	from := date(2026, time.September, 10) // четверг
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
	from := date(2026, time.September, 8)                                  // вторник
	newStart := time.Date(2026, time.September, 9, 5, 0, 0, 0, time.UTC)   // среда, 10:00 Алматы
	newDate := date(2026, time.September, 9)

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

// Перенос на более ранний день недели двигает cut назад: старая среда того же
// правила обязана уйти, иначе ReassignToRule упрётся в уникальный индекс.
func TestRetime_MovingEarlierPullsCutBack(t *testing.T) {
	repo := new(mockRecurrenceRepo)
	svc := service.NewRecurrenceService(repo)
	rule := retimeRule()
	from := date(2026, time.September, 10)                                // четверг
	newStart := time.Date(2026, time.September, 8, 5, 0, 0, 0, time.UTC)  // вторник
	cut := date(2026, time.September, 8)

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
```

- [ ] **Step 2: Запустить тесты и убедиться, что они падают**

Run: `go test ./service/ -run TestRetime -v`
Expected: FAIL — `svc.Retime undefined (type service.RecurrenceService has no field or method Retime)`

- [ ] **Step 3: Реализовать `Retime`**

В `service/recurrence.go`, в интерфейс `RecurrenceService`:

```go
	// Retime переносит серию, взяв за образец вхождение occurrenceID.
	//
	// Вызывающий обязан обновить свою строку (время, заметки, статус) ДО вызова:
	// Retime переставляет только rule_id/occurrence_date и пересобирает будущее.
	//
	// scope = following — правило разрезается, вхождение уходит в новую ветку;
	//         all       — правится правило целиком, прошедшие не трогаются.
	Retime(ctx context.Context, rule models.RecurrenceRule, occurrenceID string,
		from, newStart time.Time, duration int, scope string) error
```

Реализация (дописать в `service/recurrence.go` после `CloseRule`):

```go
func (s *recurrenceService) Retime(ctx context.Context, rule models.RecurrenceRule, occurrenceID string,
	from, newStart time.Time, duration int, scope string) error {

	if scope != scopeFollowing && scope != scopeAll {
		return fmt.Errorf("scope %q: %w", scope, ErrBadRequest)
	}

	normalized, err := NormalizeSeriesStart(rule, from, newStart)
	if err != nil {
		return err
	}
	loc, err := time.LoadLocation(rule.TZ)
	if err != nil {
		return fmt.Errorf("timezone %q: %w", rule.TZ, ErrBadRequest)
	}
	local := normalized.In(loc)
	newDate := dayOf(local)
	timeLocal := local.Format("15:04")

	byweekday := rule.ByWeekday
	if rule.Freq == "weekly" && isoWeekday(newDate) != isoWeekday(from) {
		byweekday = swapWeekday(rule, isoWeekday(from), isoWeekday(newDate))
	}

	// cut — дата, с которой серия пересобирается. При переезде на более ранний
	// день недели это новая дата: старое вхождение того дня обязано уйти.
	cut := dayOf(from)
	if newDate.Before(cut) {
		cut = newDate
	}

	targetID := rule.ID
	if scope == scopeFollowing {
		newRule, err := s.repo.Split(ctx, rule.ID, cut, timeLocal, duration, byweekday)
		if err != nil {
			return err
		}
		targetID = newRule.ID
	} else {
		if err := s.repo.UpdateTiming(ctx, rule.ID, timeLocal, duration, byweekday); err != nil {
			return err
		}
		// Границу откатываем ДО разрушающего удаления. Обрыв на любом следующем
		// шаге оставит правило в выборке DueForMaterialization, и ночная джоба
		// долечит; обратный порядок оставил бы дыру навсегда — уже без ошибки
		// в коде. Дублей не будет: (rule_id, occurrence_date) уникален.
		if err := s.repo.SetMaterializedUntil(ctx, rule.ID, cut); err != nil {
			return err
		}
	}

	// Чистим ИСХОДНОЕ правило: при following целевое только что создано и пусто,
	// а будущее старой ветки иначе живёт параллельно новой. Граница
	// включительная, поэтому в after (семантика строгого >) уходит cut − 1 день.
	if err := s.repo.DeleteFutureByRule(ctx, rule.ID, cut.AddDate(0, 0, -1), occurrenceID); err != nil {
		return err
	}
	if err := s.repo.ReassignToRule(ctx, occurrenceID, targetID, newDate); err != nil {
		return err
	}
	_, err = s.Materialize(ctx, targetID, time.Now().Add(RecurrenceHorizon))
	return err
}
```

- [ ] **Step 4: Запустить тесты и убедиться, что они проходят**

Run: `go test ./service/ -run TestRetime -v`
Expected: PASS, шесть тестов

- [ ] **Step 5: Переписать `updateSeries` в `service/lesson.go`**

Заменить тело `updateSeries` целиком:

```go
func (s *lessonService) updateSeries(ctx context.Context, lesson models.Lesson, req models.UpdateLessonRequest, tutorID, scope string) (models.Lesson, error) {
	rule, err := s.recurrence.GetRule(ctx, *lesson.RuleID)
	if err != nil {
		return models.Lesson{}, err
	}
	// Серию задают день недели и время; выбранная в форме неделя роли не играет.
	// Без нормализации строка урока и occurrence_date разъедутся.
	req.ScheduledAt, err = NormalizeSeriesStart(rule, *lesson.OccurrenceDate, req.ScheduledAt)
	if err != nil {
		return models.Lesson{}, err
	}

	// Своя строка идёт первой: если она прошла, а Retime упал, урок стоит на
	// новом времени при нетронутой серии — состояние видимое и чинится повтором.
	updated, err := s.repo.Update(ctx, lesson.ID, req)
	if err != nil {
		return models.Lesson{}, err
	}
	if err := s.recurrence.Retime(ctx, rule, lesson.ID, *lesson.OccurrenceDate,
		req.ScheduledAt, req.DurationMinutes, scope); err != nil {
		return models.Lesson{}, err
	}
	globalCalendarCache.Invalidate(tutorID)
	return updated, nil
}
```

- [ ] **Step 6: Переписать `Update` в `service/event.go`, удалить `applyToSeries`**

Заменить ветку серии в `eventService.Update`:

```go
	if scope != scopeOne && current.RuleID != nil && current.OccurrenceDate != nil {
		rule, err := s.recurrence.GetRule(ctx, *current.RuleID)
		if err != nil {
			return models.Event{}, err
		}
		req.StartsAt, err = NormalizeSeriesStart(rule, *current.OccurrenceDate, req.StartsAt)
		if err != nil {
			return models.Event{}, err
		}
		e, err := s.repo.Update(ctx, id, tutorID, req)
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Event{}, fmt.Errorf("event: %w", ErrNotFound)
		}
		if err != nil {
			return e, err
		}
		if err := s.recurrence.Retime(ctx, rule, current.ID, *current.OccurrenceDate,
			req.StartsAt, req.DurationMinutes, scope); err != nil {
			return models.Event{}, err
		}
		return e, nil
	}
```

Оставшуюся часть `Update` (ветка `scope == one` с `MarkOverride`) не трогать. Функцию `applyToSeries` удалить целиком.

- [ ] **Step 7: Переписать `TestLessonUpdate_ScopeFollowing` и `TestLessonUpdate_ScopeAll`**

Снять `t.Skip` из Задачи 2 и привести оба теста к новым вызовам. `TestLessonUpdate_ScopeAll`:

```go
func TestLessonUpdate_ScopeAll(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	ruleRepo := new(mockRecurrenceRepo)
	svc := scopedSvc(lessonRepo, ruleRepo)

	lesson := seriesLesson()
	rule := models.RecurrenceRule{
		ID: "rule-1", Freq: "weekly", IntervalN: 1, ByWeekday: []int{2},
		TimeLocal: "17:00", TZ: "Asia/Almaty", DurationMinutes: 60,
		StartsOn:          date(2026, time.September, 1),
		MaterializedUntil: date(2027, time.March, 5),
	}

	lessonRepo.On("GetByIDForTutor", mock.Anything, lessonID, tutorID).Return(lesson, nil)
	ruleRepo.On("GetByID", mock.Anything, "rule-1").Return(rule, nil)
	lessonRepo.On("Update", mock.Anything, lessonID, mock.Anything).Return(expectedLesson, nil)
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
}
```

`TestLessonUpdate_ScopeFollowing` — то же, но с `ruleRepo.On("Split", ...)` вместо `UpdateTiming`/`SetMaterializedUntil` и `ReassignToRule` на `"rule-2"`; `lessonRepo.AssertNotCalled(t, "ReassignToRule")`, потому что метод оттуда уехал.

- [ ] **Step 8: Добавить тест на серийную ветку событий**

Существующие тесты в `scope_test.go:232-296` покрывают у событий только `scope=one`. А дефект A живёт в проде именно у событий — ветка `all` обязана иметь свой тест. Дописать в `service/scope_test.go`:

```go
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
		StartsOn:          date(2026, time.September, 1),
		MaterializedUntil: date(2027, time.March, 5),
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
```

- [ ] **Step 9: Прогнать всё**

Run: `go build ./... && go test ./...`
Expected: PASS

- [ ] **Step 10: Коммит**

```bash
git add service/
git commit -m "fix(recurrence): scope=all больше не сносит будущее, серия умеет менять день недели

Retime собирает арифметику правила в одном месте: пересчёт byweekday,
Split/UpdateTiming, откат materialized_until перед разрушающим удалением.
applyToSeries в event.go удалён — обе копии логики схлопнуты в одну."
```

---

### Task 4: интеграционный тест переноса серии

Дефект A на моках не воспроизводится в принципе: фикстура правила создаётся с нулевым `MaterializedUntil`, и ранний выход `Materialize` не срабатывает. Без живой БД проверить нечего.

**Files:**
- Create: `repository/recurrence_integration_test.go`

**Interfaces:**
- Consumes: `service.NewRecurrenceService`, `repository.NewRecurrenceRepository`, `testPool` из `repository/whiteboard_elements_integration_test.go:33`

- [ ] **Step 1: Написать тест**

Создать `repository/recurrence_integration_test.go`:

```go
//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"tutorgo/repository"
	"tutorgo/service"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Перенос серии на живой БД. Запуск: make test-integration.
//
// На моках эти сценарии зелёные и на сломанном коде: ранний выход Materialize
// зависит от реального materialized_until, а фикстура ставит его нулевым.

// seedSeries создаёт репетитора, курс, правило и count еженедельных уроков
// начиная с first. Каскад от tutors сносит всё остальное.
func seedSeries(t *testing.T, pool *pgxpool.Pool, first time.Time, count int) (tutorID, courseID, ruleID string) {
	ctx := context.Background()
	email := fmt.Sprintf("retime-%d@example.com", time.Now().UnixNano())

	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO tutors (email, password_hash, first_name, last_name)
		 VALUES ($1, 'x', 'Test', 'Tutor') RETURNING id`, email).Scan(&tutorID))
	t.Cleanup(func() {
		_, err := pool.Exec(context.Background(), `DELETE FROM tutors WHERE id=$1`, tutorID)
		assert.NoError(t, err)
	})

	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO courses (tutor_id, subject, started_at)
		 VALUES ($1, 'Английский', $2) RETURNING id`, tutorID, first).Scan(&courseID))

	horizon := first.AddDate(0, 0, 7*count)
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO recurrence_rules
		     (tutor_id, freq, interval_n, byweekday, time_local, tz, duration_minutes,
		      starts_on, materialized_until)
		 VALUES ($1, 'weekly', 1, $2::smallint[], '17:00', 'Asia/Almaty', 60, $3::date, $4::date)
		 RETURNING id`,
		tutorID, []int{int(first.Weekday()+6)%7 + 1}, first, horizon).Scan(&ruleID))

	for i := 0; i < count; i++ {
		at := first.AddDate(0, 0, 7*i)
		_, err := pool.Exec(ctx,
			`INSERT INTO lessons (course_id, scheduled_at, duration_minutes, notes, status, rule_id, occurrence_date)
			 VALUES ($1, $2, 60, '', 'scheduled', $3::uuid, $2::date)`,
			courseID, at, ruleID)
		require.NoError(t, err)
	}
	return tutorID, courseID, ruleID
}

// «Изменить все» на пятом вхождении: первые четыре остаются на старом времени,
// с пятого по десятое встают на новое, ни одного не потеряно и не задвоено.
func TestRetime_ScopeAllKeepsEveryOccurrence(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	// Ближайший четверг в будущем: прошедшие уроки перенос не трогает.
	first := nextWeekday(time.Now().AddDate(0, 0, 1), time.Thursday).
		Truncate(24 * time.Hour).Add(12 * time.Hour)
	_, courseID, ruleID := seedSeries(t, pool, first, 10)

	repo := repository.NewRecurrenceRepository(pool)
	svc := service.NewRecurrenceService(repo)
	rule, err := repo.GetByID(ctx, ruleID)
	require.NoError(t, err)

	fifth := first.AddDate(0, 0, 7*4)
	newStart := time.Date(fifth.Year(), fifth.Month(), fifth.Day(), 5, 0, 0, 0, time.UTC) // 10:00 Алматы

	var fifthID string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM lessons WHERE rule_id=$1::uuid AND occurrence_date=$2::date`,
		ruleID, fifth).Scan(&fifthID))

	require.NoError(t, svc.Retime(ctx, rule, fifthID, fifth.Truncate(24*time.Hour), newStart, 60, "all"))

	var total, atNewTime int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*),
		        count(*) FILTER (WHERE (scheduled_at AT TIME ZONE 'Asia/Almaty')::time = '10:00')
		 FROM lessons WHERE course_id=$1`, courseID).Scan(&total, &atNewTime))

	assert.Equal(t, 10, total, "ни одно вхождение не потеряно и не задвоено")
	assert.Equal(t, 6, atNewTime, "с пятого по десятое — на новом времени")
}

// Перенос серии со своего дня недели на другой: byweekday правила меняется,
// все будущие вхождения встают на новый день, прошедшие остаются на старом.
func TestRetime_ScopeAllMovesWeekday(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	first := nextWeekday(time.Now().AddDate(0, 0, 1), time.Thursday).
		Truncate(24 * time.Hour).Add(12 * time.Hour)
	_, courseID, ruleID := seedSeries(t, pool, first, 6)

	repo := repository.NewRecurrenceRepository(pool)
	svc := service.NewRecurrenceService(repo)
	rule, err := repo.GetByID(ctx, ruleID)
	require.NoError(t, err)

	second := first.AddDate(0, 0, 7)
	// Суббота той же недели.
	newStart := time.Date(second.Year(), second.Month(), second.Day(), 5, 0, 0, 0, time.UTC).AddDate(0, 0, 2)

	var secondID string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM lessons WHERE rule_id=$1::uuid AND occurrence_date=$2::date`,
		ruleID, second).Scan(&secondID))

	require.NoError(t, svc.Retime(ctx, rule, secondID, second.Truncate(24*time.Hour), newStart, 60, "all"))

	var byweekday []int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT byweekday FROM recurrence_rules WHERE id=$1`, ruleID).Scan(&byweekday))
	assert.Equal(t, []int{6}, byweekday, "правило переехало на субботу")

	var onSaturday, onThursday int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FILTER (WHERE EXTRACT(ISODOW FROM occurrence_date) = 6),
		        count(*) FILTER (WHERE EXTRACT(ISODOW FROM occurrence_date) = 4)
		 FROM lessons WHERE course_id=$1`, courseID).Scan(&onSaturday, &onThursday))
	assert.Positive(t, onSaturday)
	assert.Equal(t, 1, onThursday, "первое вхождение — в прошлом относительно cut, осталось на четверге")
}

// Вручную перенесённое вхождение переживает «изменить все»: is_override —
// единственное, что отделяет осознанный перенос от строки, которую можно снести
// и пересоздать по расписанию.
func TestRetime_ScopeAllSparesOverriddenOccurrence(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	first := nextWeekday(time.Now().AddDate(0, 0, 1), time.Thursday).
		Truncate(24 * time.Hour).Add(12 * time.Hour)
	_, courseID, ruleID := seedSeries(t, pool, first, 6)

	// Третье вхождение репетитор подвинул руками на день позже.
	third := first.AddDate(0, 0, 14)
	var thirdID string
	require.NoError(t, pool.QueryRow(ctx,
		`UPDATE lessons SET is_override = TRUE, scheduled_at = scheduled_at + interval '1 day'
		 WHERE rule_id=$1::uuid AND occurrence_date=$2::date RETURNING id`,
		ruleID, third).Scan(&thirdID))

	repo := repository.NewRecurrenceRepository(pool)
	svc := service.NewRecurrenceService(repo)
	rule, err := repo.GetByID(ctx, ruleID)
	require.NoError(t, err)

	newStart := time.Date(first.Year(), first.Month(), first.Day(), 5, 0, 0, 0, time.UTC)
	var firstID string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM lessons WHERE rule_id=$1::uuid AND occurrence_date=$2::date`,
		ruleID, first).Scan(&firstID))

	require.NoError(t, svc.Retime(ctx, rule, firstID, first.Truncate(24*time.Hour), newStart, 60, "all"))

	var stillOverridden bool
	var localTime string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT is_override, (scheduled_at AT TIME ZONE 'Asia/Almaty')::time::text
		 FROM lessons WHERE id=$1`, thirdID).Scan(&stillOverridden, &localTime))
	assert.True(t, stillOverridden, "перенос пережил правку всей серии")
	assert.Equal(t, "17:00:00", localTime, "и остался на своём времени, а не на новом")

	var total int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM lessons WHERE course_id=$1`, courseID).Scan(&total))
	assert.Equal(t, 6, total, "пересоздание не задвоило перенесённое вхождение")
}

// «Это и все следующие» на удаление: текущее вхождение отменяется (строка
// остаётся тумбстоуном — удали её, и материализация вернёт урок на свободную
// дату), последующие удаляются, правило закрывается, джоба их не возвращает.
func TestDeleteFollowing_CancelsCurrentAndClosesRule(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	first := nextWeekday(time.Now().AddDate(0, 0, 1), time.Thursday).
		Truncate(24 * time.Hour).Add(12 * time.Hour)
	tutorID, courseID, ruleID := seedSeries(t, pool, first, 8)

	third := first.AddDate(0, 0, 14)
	var thirdID string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM lessons WHERE rule_id=$1::uuid AND occurrence_date=$2::date`,
		ruleID, third).Scan(&thirdID))

	svc := service.NewLessonService(repository.NewLessonRepository(pool),
		repository.NewCourseRepository(pool), repository.NewPaymentRepository(pool),
		service.NewRecurrenceService(repository.NewRecurrenceRepository(pool)))

	require.NoError(t, svc.Delete(ctx, thirdID, tutorID, "following"))

	var status string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status FROM lessons WHERE id=$1`, thirdID).Scan(&status))
	assert.Equal(t, "cancelled", status, "текущее вхождение отменено, а не удалено")

	var later int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM lessons WHERE course_id=$1 AND occurrence_date > $2::date`,
		courseID, third).Scan(&later))
	assert.Zero(t, later, "последующие удалены")

	rec := service.NewRecurrenceService(repository.NewRecurrenceRepository(pool))
	_, err := rec.ExtendAll(ctx, time.Now().Add(service.RecurrenceHorizon))
	require.NoError(t, err)

	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM lessons WHERE course_id=$1 AND occurrence_date > $2::date`,
		courseID, third).Scan(&later))
	assert.Zero(t, later, "джоба не вернула удалённое: правило закрыто")
}

// nextWeekday — ближайший день недели wd не раньше from.
func nextWeekday(from time.Time, wd time.Weekday) time.Time {
	for d := 0; d < 7; d++ {
		if day := from.AddDate(0, 0, d); day.Weekday() == wd {
			return day
		}
	}
	return from
}
```

- [ ] **Step 2: Запустить и убедиться, что тесты проходят**

Run: `make test-integration`
Expected: PASS. Без `TEST_DB_URL` тесты пропускаются — тогда задай переменную:
`TEST_DB_URL=postgres://dev:dev@localhost:5432/tutorgo_dev make test-integration`

Если `TestRetime_ScopeAllKeepsEveryOccurrence` даёт `total < 10` — откат watermark не сработал: проверь, что `SetMaterializedUntil` в `Retime` стоит **до** `DeleteFutureByRule`.

- [ ] **Step 3: Коммит**

```bash
git add repository/recurrence_integration_test.go
git commit -m "test(recurrence): интеграционные тесты переноса серии на живой БД"
```

---

### Task 5: `lessonColumns` — оживление серийного UI

Причина, по которой поля забыли в четырёх местах, а у событий — ни в одном: `repository/event.go:14` держит `eventColumns` одной константой.

**Files:**
- Modify: `repository/lesson.go:57` (`GetByCourse`), `:77` (`GetByCoursePaged`), `:105` (`GetByID`), `:154` (`GetByIDForTutor`), `:248` (`GetByPeriod`); шапка файла — константа и `scanLesson`

**Interfaces:**
- Produces: `models.Lesson.RuleID` и `.OccurrenceDate` заполнены во всех выборках урока

- [ ] **Step 1: Добавить константу и скан**

В `repository/lesson.go` после объявления `lessonRepository` (перед `Create`):

```go
// Одна константа на все выборки урока: россыпь SQL — ровно та причина, по
// которой rule_id и occurrence_date забыли в четырёх местах, а у событий
// (см. eventColumns) не забыли ни в одном. Алиас `l` обязателен и там, где
// джойна нет, иначе константу не переиспользовать.
const lessonColumns = `l.id, l.course_id, l.scheduled_at, l.duration_minutes,
                       l.status, l.notes, l.rule_id, l.occurrence_date`

func scanLesson(row interface{ Scan(...any) error }) (models.Lesson, error) {
	var l models.Lesson
	err := row.Scan(&l.ID, &l.CourseID, &l.ScheduledAt, &l.DurationMinutes,
		&l.Status, &l.Notes, &l.RuleID, &l.OccurrenceDate)
	return l, err
}
```

- [ ] **Step 2: Перевести пять выборок**

```go
func (r *lessonRepository) GetByCourse(ctx context.Context, courseID string) ([]models.Lesson, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+lessonColumns+` FROM lessons l
		 WHERE l.course_id = $1 ORDER BY l.scheduled_at`, courseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	lessons := []models.Lesson{}
	for rows.Next() {
		lesson, err := scanLesson(rows)
		if err != nil {
			return nil, err
		}
		lessons = append(lessons, lesson)
	}
	return lessons, rows.Err()
}
```

`GetByCoursePaged` — тот же `SELECT `+lessonColumns+` FROM lessons l WHERE l.course_id = $1 ORDER BY l.scheduled_at DESC LIMIT $2 OFFSET $3` плюс `scanLesson(rows)` в цикле; запрос `COUNT(*)` не трогать.

`GetByID` — `SELECT `+lessonColumns+` FROM lessons l WHERE l.id = $1`, вернуть `scanLesson(r.pool.QueryRow(...))`.

`GetByIDForTutor` — `SELECT `+lessonColumns+` FROM lessons l JOIN courses c ON c.id = l.course_id WHERE l.id = $1 AND c.tutor_id = $2`, вернуть `scanLesson(...)`.

`GetByPeriod` — `SELECT `+lessonColumns+` FROM lessons l JOIN courses c ON c.id = l.course_id WHERE l.course_id = $1 AND c.tutor_id = $2 AND l.scheduled_at >= $3::timestamptz AND l.scheduled_at < $4::timestamptz ORDER BY l.scheduled_at ASC` плюс `scanLesson(rows)`.

- [ ] **Step 3: Прогнать тесты**

Run: `go build ./... && go test ./...`
Expected: PASS

- [ ] **Step 4: Проверить руками, что поле доехало**

Run: подними `make run`, залогинься, открой `/courses/:id` с еженедельной серией и нажми карандаш на любом уроке.
Expected: появляется диалог «Изменить повторяющееся занятие» с тремя вариантами. До этой задачи он не открывался никогда.

- [ ] **Step 5: Коммит**

```bash
git add repository/lesson.go
git commit -m "fix(lessons): rule_id и occurrence_date во всех выборках урока

Одна константа колонок по образцу eventColumns: поля нельзя забыть
в пятом месте. Серийный UI на странице курса оживает без правок фронта."
```

---

### Task 6: правила курса при архивации и удалении уроков

**Files:**
- Modify: `repository/lesson.go` (интерфейс + два новых метода)
- Modify: `service/lesson.go:18-28` (интерфейс), `:337` (`DeleteByCourse`), новый метод
- Modify: `service/course.go:25-32` (структура и конструктор), `:78` (`Delete`)
- Modify: `router/router.go:55` (перенести ниже `:58`)
- Modify: `service/lesson_test.go` (моки), `service/course_test.go:91` (конструктор), `handlers/mocks_test.go`

**Interfaces:**
- Produces (`repository.LessonRepository`):
  - `GetRuleIDsByCourse(ctx context.Context, courseID string) ([]string, error)`
  - `DeleteFutureByCourse(ctx context.Context, courseID, tutorID string) error`
- Produces (`service.LessonService`): `ArchiveCourseSchedule(ctx context.Context, courseID, tutorID string) error`
- Produces (`service`): `NewCourseService(repo repository.CourseRepository, studentRepo repository.StudentRepository, schedule courseSchedule) CourseService`

- [ ] **Step 1: Написать падающий тест**

Дописать в `service/lesson_test.go`:

```go
// Архивация закрывает серии курса и убирает будущие уроки: завершённые
// остаются — это ровно то, что обещает диалог архивации.
//
// Порядок обязателен: правило связано с курсом только через lessons.rule_id,
// и после удаления уроков связь не восстановить.
func TestArchiveCourseSchedule_ClosesRulesBeforeDeleting(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	ruleRepo := new(mockRecurrenceRepo)
	svc := scopedSvc(lessonRepo, ruleRepo)

	var order []string
	lessonRepo.On("GetRuleIDsByCourse", mock.Anything, courseID).
		Run(func(mock.Arguments) { order = append(order, "read") }).
		Return([]string{"rule-1"}, nil)
	ruleRepo.On("SetEndsOn", mock.Anything, "rule-1", mock.Anything).
		Run(func(mock.Arguments) { order = append(order, "close") }).Return(nil)
	lessonRepo.On("DeleteFutureByCourse", mock.Anything, courseID, tutorID).
		Run(func(mock.Arguments) { order = append(order, "delete") }).Return(nil)

	require.NoError(t, svc.ArchiveCourseSchedule(context.Background(), courseID, tutorID))

	require.Equal(t, []string{"read", "close", "delete"}, order)
	lessonRepo.AssertExpectations(t)
}

// «Удалить все уроки» удаляет и правила: уроков не остаётся, шаблона для
// материализации у правила нет, хранить его незачем — иначе копятся сироты.
func TestDeleteByCourse_RemovesRules(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	ruleRepo := new(mockRecurrenceRepo)
	courseRepo := new(mockCourseRepo)
	svc := service.NewLessonService(lessonRepo, courseRepo, new(mockPaymentRepo),
		service.NewRecurrenceService(ruleRepo))

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(models.Course{ID: courseID}, nil)
	lessonRepo.On("GetRuleIDsByCourse", mock.Anything, courseID).Return([]string{"rule-1", "rule-2"}, nil)
	lessonRepo.On("DeleteByCourse", mock.Anything, courseID, tutorID).Return(nil)
	ruleRepo.On("Delete", mock.Anything, "rule-1").Return(nil)
	ruleRepo.On("Delete", mock.Anything, "rule-2").Return(nil)

	require.NoError(t, svc.DeleteByCourse(context.Background(), courseID, tutorID))

	ruleRepo.AssertExpectations(t)
}
```

Дописать моки в `service/lesson_test.go`:

```go
func (m *mockLessonRepo) GetRuleIDsByCourse(ctx context.Context, courseID string) ([]string, error) {
	args := m.Called(ctx, courseID)
	return args.Get(0).([]string), args.Error(1)
}
func (m *mockLessonRepo) DeleteFutureByCourse(ctx context.Context, courseID, tutorID string) error {
	return m.Called(ctx, courseID, tutorID).Error(0)
}
```

`mockRecurrenceRepo.Delete` и `.SetEndsOn` дописывать не надо — они уже есть в `service/series_test.go:20` и `service/scope_test.go:36`.

- [ ] **Step 2: Запустить тесты и убедиться, что они падают**

Run: `go test ./service/ -run "TestArchiveCourseSchedule|TestDeleteByCourse" -v`
Expected: FAIL — `svc.ArchiveCourseSchedule undefined`

- [ ] **Step 3: Добавить методы репозитория**

В интерфейс `LessonRepository`:

```go
	GetRuleIDsByCourse(ctx context.Context, courseID string) ([]string, error)
	DeleteFutureByCourse(ctx context.Context, courseID, tutorID string) error
```

Реализации:

```go
// GetRuleIDsByCourse — единственный путь от курса к его правилам: прямой связи
// в схеме нет, только через lessons.rule_id. Поэтому читать надо ДО удаления
// уроков, иначе связь потеряна безвозвратно.
func (r *lessonRepository) GetRuleIDsByCourse(ctx context.Context, courseID string) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT rule_id::text FROM lessons
		 WHERE course_id = $1 AND rule_id IS NOT NULL`, courseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DeleteFutureByCourse убирает то, что ещё не состоялось. Завершённые,
// отменённые и пропущенные остаются: диалог архивации обещает именно это.
func (r *lessonRepository) DeleteFutureByCourse(ctx context.Context, courseID, tutorID string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM lessons
		 USING courses
		 WHERE lessons.course_id = $1
		   AND lessons.course_id = courses.id
		   AND courses.tutor_id = $2
		   AND lessons.status = 'scheduled'
		   AND lessons.scheduled_at > NOW()`,
		courseID, tutorID)
	return err
}
```

- [ ] **Step 4: Добавить `ArchiveCourseSchedule` и переписать `DeleteByCourse`**

В интерфейс `LessonService` добавить строку `ArchiveCourseSchedule(ctx context.Context, courseID, tutorID string) error`.

```go
// ArchiveCourseSchedule закрывает серии курса и убирает будущие уроки.
// Завершённые остаются — так обещает диалог архивации, и на них висят
// посещаемость, платежи и доски.
//
// Правило закрываем, а не удаляем: у прошедших уроков rule_id остаётся, и
// история занятий сохраняет признак серии. Из DueForMaterialization закрытое
// правило выпадает по ends_on > CURRENT_DATE.
func (s *lessonService) ArchiveCourseSchedule(ctx context.Context, courseID, tutorID string) error {
	ruleIDs, err := s.repo.GetRuleIDsByCourse(ctx, courseID)
	if err != nil {
		return err
	}
	for _, id := range ruleIDs {
		// CloseRule ставит ends_on = at − 1, то есть вчера: всё, что позже,
		// серии больше не принадлежит.
		if err := s.recurrence.CloseRule(ctx, id, time.Now()); err != nil {
			return err
		}
	}
	if err := s.repo.DeleteFutureByCourse(ctx, courseID, tutorID); err != nil {
		return err
	}
	globalCalendarCache.Invalidate(tutorID)
	return nil
}

func (s *lessonService) DeleteByCourse(ctx context.Context, courseID string, tutorID string) error {
	if _, err := s.courseRepo.GetByID(ctx, courseID, tutorID); err != nil {
		return fmt.Errorf("course: %w", ErrNotFound)
	}
	// Связь правила с курсом — только через lessons.rule_id: читаем до удаления.
	ruleIDs, err := s.repo.GetRuleIDsByCourse(ctx, courseID)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteByCourse(ctx, courseID, tutorID); err != nil {
		return err
	}
	// Уроков не осталось — InsertOccurrences не найдёт шаблона, и правило
	// становится мусором, который ночная джоба будет сканировать вечно.
	for _, id := range ruleIDs {
		if err := s.recurrence.DeleteRule(ctx, id); err != nil {
			return err
		}
	}
	globalCalendarCache.Invalidate(tutorID)
	return nil
}
```

- [ ] **Step 5: Подключить архивацию к `courseService`**

В `service/course.go`:

```go
// courseSchedule — узкий выход к расписанию: courseService собран из
// courseRepo и studentRepo и до уроков с правилами сам не достаёт.
type courseSchedule interface {
	ArchiveCourseSchedule(ctx context.Context, courseID, tutorID string) error
}

type courseService struct {
	repo        repository.CourseRepository
	studentRepo repository.StudentRepository
	schedule    courseSchedule
}

func NewCourseService(repo repository.CourseRepository, studentRepo repository.StudentRepository, schedule courseSchedule) CourseService {
	return &courseService{repo: repo, studentRepo: studentRepo, schedule: schedule}
}

func (s *courseService) Delete(ctx context.Context, id string, tutorID string) error {
	if _, err := s.repo.GetByID(ctx, id, tutorID); err != nil {
		return fmt.Errorf("course: %w", ErrNotFound)
	}
	if err := s.repo.Delete(ctx, id, tutorID); err != nil {
		return err
	}
	// Архивация обещает: завершённые остаются, будущие уходят. Без этого
	// правило курса живёт дальше и продолжает материализовать уроки.
	return s.schedule.ArchiveCourseSchedule(ctx, id, tutorID)
}
```

- [ ] **Step 6: Переставить проводку в `router/router.go`**

Строку `courseService := service.NewCourseService(courseRepo, studentRepo)` (`:55`) удалить и вставить **после** строки создания `lessonService` (`:58`) в виде:

```go
	// Ниже lessonService: архивация курса ходит к нему за закрытием серий.
	// Цикла нет — lessonService зависит от courseRepo, а не от courseService.
	courseService := service.NewCourseService(courseRepo, studentRepo, lessonService)
```

- [ ] **Step 7: Починить `service/course_test.go`**

`service/course_test.go:91` — добавить третий аргумент. Простейший вариант: заглушка прямо в тестовом файле.

```go
type stubSchedule struct{}

func (stubSchedule) ArchiveCourseSchedule(context.Context, string, string) error { return nil }
```

и `return service.NewCourseService(courseRepo, studentRepo, stubSchedule{})`.

- [ ] **Step 8: Синхронизировать `handlers/mocks_test.go`**

Добавить в `mockLessonService`:

```go
func (m *mockLessonService) ArchiveCourseSchedule(ctx context.Context, courseID, tutorID string) error {
	return m.Called(ctx, courseID, tutorID).Error(0)
}
```

- [ ] **Step 9: Прогнать всё**

Run: `go build ./... && go test ./...`
Expected: PASS

- [ ] **Step 10: Коммит**

```bash
git add repository/lesson.go service/ router/router.go handlers/mocks_test.go
git commit -m "fix(courses): архивация закрывает серии и убирает будущие уроки

DeleteByCourse заодно удаляет правила курса — иначе копятся сироты,
которые ночная джоба сканирует вечно."
```

---

### Task 7: интеграционный тест архивации

**Files:**
- Modify: `repository/recurrence_integration_test.go` (дописать)

- [ ] **Step 1: Написать тест**

Дописать в `repository/recurrence_integration_test.go`:

```go
// Архивация курса: будущие уроки уходят, завершённые остаются, а ночная джоба
// не возвращает удалённое — правило закрыто и выпало из DueForMaterialization.
func TestArchiveCourseSchedule_JobDoesNotBringLessonsBack(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	// Первое вхождение в прошлом: часть уроков успела стать completed.
	first := time.Now().AddDate(0, 0, -21).Truncate(24 * time.Hour).Add(12 * time.Hour)
	tutorID, courseID, ruleID := seedSeries(t, pool, first, 10)

	_, err := pool.Exec(ctx,
		`UPDATE lessons SET status='completed' WHERE course_id=$1 AND scheduled_at < NOW()`, courseID)
	require.NoError(t, err)

	lessonRepo := repository.NewLessonRepository(pool)
	svc := service.NewLessonService(lessonRepo, repository.NewCourseRepository(pool),
		repository.NewPaymentRepository(pool),
		service.NewRecurrenceService(repository.NewRecurrenceRepository(pool)))

	require.NoError(t, svc.ArchiveCourseSchedule(ctx, courseID, tutorID))

	var future, past int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FILTER (WHERE scheduled_at > NOW()),
		        count(*) FILTER (WHERE scheduled_at < NOW())
		 FROM lessons WHERE course_id=$1`, courseID).Scan(&future, &past))
	assert.Zero(t, future, "будущих уроков не осталось")
	assert.Positive(t, past, "завершённые остались")

	// Ночная джоба: правило закрыто, возвращать ей нечего.
	rec := service.NewRecurrenceService(repository.NewRecurrenceRepository(pool))
	_, err = rec.ExtendAll(ctx, time.Now().Add(service.RecurrenceHorizon))
	require.NoError(t, err)

	var afterJob int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM lessons WHERE course_id=$1 AND scheduled_at > NOW()`,
		courseID).Scan(&afterJob))
	assert.Zero(t, afterJob, "джоба не вернула уроки архивированного курса")

	var endsOn *time.Time
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT ends_on FROM recurrence_rules WHERE id=$1`, ruleID).Scan(&endsOn))
	require.NotNil(t, endsOn, "правило закрыто, а не оставлено бессрочным")
}

// «Удалить все уроки» не оставляет правил-сирот.
func TestDeleteByCourse_LeavesNoOrphanRules(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	first := time.Now().AddDate(0, 0, 7).Truncate(24 * time.Hour).Add(12 * time.Hour)
	tutorID, courseID, ruleID := seedSeries(t, pool, first, 5)

	svc := service.NewLessonService(repository.NewLessonRepository(pool),
		repository.NewCourseRepository(pool), repository.NewPaymentRepository(pool),
		service.NewRecurrenceService(repository.NewRecurrenceRepository(pool)))

	require.NoError(t, svc.DeleteByCourse(ctx, courseID, tutorID))

	var rules int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM recurrence_rules WHERE id=$1`, ruleID).Scan(&rules))
	assert.Zero(t, rules, "правило удалено вместе с уроками")
}
```

- [ ] **Step 2: Запустить**

Run: `make test-integration`
Expected: PASS

- [ ] **Step 3: Коммит**

```bash
git add repository/recurrence_integration_test.go
git commit -m "test(courses): интеграционные тесты архивации и удаления уроков курса"
```

---

### Task 8: попап урока в календаре

**Выполняется параллельно с Задачами 1–7** — ни одного общего файла с бэкендом.

**Files:**
- Modify: `frontend/src/app/(dashboard)/calendar/page.tsx:123` (`extendedProps`), `:233-247` (`handleEventClick`)
- Modify: `frontend/src/components/lessons/LessonQuickPopover.tsx` (весь)
- Modify: `frontend/src/lib/hooks/useCalendar.ts:8-21` (`useUpdateLessonStatus`)
- Modify: `frontend/src/app/(dashboard)/courses/page.tsx:140` (текст диалога)

**Interfaces:**
- Consumes: `CalendarLesson.rule_id` (уже в типе, `types/api.ts:93`), `RecurrenceScopeDialog`, `useDeleteLesson`
- Produces: `QuickLesson.ruleId?: string`

- [ ] **Step 1: Прокинуть `rule_id` через `extendedProps`**

`calendar/page.tsx`, блок `extendedProps` для урока (`:123`), добавить строку после `durationMinutes`:

```ts
          ruleId:          l.rule_id ?? null,
```

- [ ] **Step 2: Перестать выбрасывать поле в `handleEventClick`**

`calendar/page.tsx:233-247`, в объект `lesson:` добавить:

```ts
        ruleId:          (p.ruleId as string | null) ?? undefined,
```

- [ ] **Step 3: Научить `useUpdateLessonStatus` области**

`frontend/src/lib/hooks/useCalendar.ts:8-21` — привести к форме `useUpdateLesson` (единственный вызывающий переписывается в Шаге 4):

```ts
export function useUpdateLessonStatus(id: string) {
  const qc = useQueryClient()
  return useMutation({
    // scope нужен только вхождению серии; одиночному уроку сервер его игнорирует.
    mutationFn: ({ data, scope }: { data: LessonUpdateInput; scope?: RecurrenceScope }) =>
      lessonsApi.update(id, data, scope),
    onMutate: ({ data }) =>
      patchCalendarEntry(
        qc, id,
        { starts_at: data.scheduled_at, duration_minutes: data.duration_minutes },
        { status: data.status, notes: data.notes },
      ).then((previousEntries) => ({ previousEntries })),
    onError:   (_err, _vars, ctx) => rollbackCalendar(qc, ctx?.previousEntries),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['calendar'] }),
  })
}
```

Импорт `RecurrenceScope` из `@/types/api` добавить в шапку файла.

- [ ] **Step 4: Переписать `LessonQuickPopover`**

Структура переносится из `EventQuickPopover.tsx:45-83` почти дословно. Изменения в `frontend/src/components/lessons/LessonQuickPopover.tsx`:

Интерфейс:

```ts
export interface QuickLesson {
  id:              string
  courseId:        string
  title:           string
  status:          LessonStatus
  notes:           string
  isGroup:         boolean
  scheduledAt:     string
  durationMinutes: number
  /** Заполнен только у вхождения серии — правка тогда спрашивает область. */
  ruleId?:         string
}
```

В `QuickLessonForm` добавить состояние и хуки:

```ts
  // datetime-local хочет местное время без зоны — toISOString() отдал бы UTC.
  const [startsAt, setStartsAt] = useState(() => {
    const d = new Date(lesson.scheduledAt)
    return new Date(d.getTime() - d.getTimezoneOffset() * 60_000).toISOString().slice(0, 16)
  })
  const [duration, setDuration] = useState(lesson.durationMinutes)

  // У вхождения серии сначала спрашиваем область; одиночный урок правится
  // сразу, лишний диалог там был бы шумом.
  const [asking, setAsking] = useState<'edit' | 'delete' | null>(null)
  const isSeries = !!lesson.ruleId

  const deleteLesson = useDeleteLesson(lesson.courseId)
```

`handleSave` разбивается на `save(scope)` и гейт:

```ts
  async function save(scope: RecurrenceScope) {
    try {
      await updateStatus.mutateAsync({
        data: {
          scheduled_at:     new Date(startsAt).toISOString(),
          duration_minutes: duration,
          status,
          notes,
        },
        scope,
      })
      if (lesson.isGroup && enrollments.length > 0) {
        await updateAttendance.mutateAsync(
          enrollments.map((e) => ({ student_id: e.student_id, status: attendanceOf(e.student_id) })),
        )
      }
      toast.success('Сохранено')
      onClose()
    } catch {
      toast.error('Ошибка сохранения')
    }
  }

  async function remove(scope: RecurrenceScope) {
    try {
      await deleteLesson.mutateAsync({ id: lesson.id, scope })
      toast.success(scope === 'one' && isSeries ? 'Урок отменён' : 'Урок удалён')
      onClose()
    } catch {
      toast.error('Не удалось удалить урок')
    }
  }

  function handleSave()   { if (isSeries) { setAsking('edit');   return } save('one') }
  function handleDelete() { if (isSeries) { setAsking('delete'); return } remove('one') }
```

В разметку, над блоком «Статус», добавить поля времени и длительности:

```tsx
        <div className="flex gap-2">
          <div className="flex-1">
            <label className="text-xs font-medium text-muted-foreground mb-1 block">Начало</label>
            <input
              type="datetime-local"
              value={startsAt}
              onChange={(e) => setStartsAt(e.target.value)}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            />
          </div>
          <div className="w-24">
            <label className="text-xs font-medium text-muted-foreground mb-1 block">Минут</label>
            <input
              type="number"
              min={15}
              step={15}
              value={duration}
              onChange={(e) => setDuration(Number(e.target.value))}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            />
          </div>
        </div>
```

В нижний блок кнопок, слева от «Перейти к курсу», добавить удаление:

```tsx
        <Button variant="ghost" size="sm" onClick={handleDelete} className="text-destructive hover:text-destructive">
          <Trash2 className="size-4" /> Удалить
        </Button>
```

И в самый конец `QuickLessonForm`, перед закрывающим `</>`:

```tsx
      <RecurrenceScopeDialog
        open={!!asking}
        action={asking ?? 'edit'}
        onPick={(scope) => (asking === 'delete' ? remove(scope) : save(scope))}
        onClose={() => setAsking(null)}
      />
```

Импорты в шапку: `Trash2` из `lucide-react`, `RecurrenceScopeDialog` из `@/components/calendar/RecurrenceScopeDialog`, `useDeleteLesson` из `@/lib/hooks/useLessons`, `RecurrenceScope` из `@/types/api`.

Заголовок с датой (`fmtDate`, `fmt(start)`) оставить как есть — он читает `lesson.scheduledAt`, то есть сохранённое значение, и до сохранения меняться не должен.

- [ ] **Step 5: Поправить текст диалога архивации**

`frontend/src/app/(dashboard)/courses/page.tsx:140` — заменить строку:

```ts
    if (!confirm(`Архивировать курс "${course.subject}"? Завершённые уроки останутся в календаре, будущие будут удалены.`)) return
```

- [ ] **Step 6: Проверить сборку и линт**

```bash
cd frontend && npm run lint && npx tsc --noEmit
```
Expected: без ошибок. Отдельного typecheck-скрипта в проекте нет — `tsc --noEmit` гоняется вручную.

- [ ] **Step 7: Проверить руками**

Run: `cd frontend && npm run dev`, открыть `/calendar`, кликнуть по уроку серии.
Expected: в попапе есть поля начала и длительности, кнопка «Удалить»; сохранение и удаление спрашивают область. Перетаскивание урока мышью область **не** спрашивает.

- [ ] **Step 8: Коммит**

```bash
git add frontend/src
git commit -m "feat(calendar): попап урока умеет время, удаление и серию

Структура перенесена из EventQuickPopover: признак серии, отложенный
вопрос об области, RecurrenceScopeDialog. Drag-and-drop по-прежнему
применяет scope=one без диалога — жест обязан оставаться безопасным."
```

---

## После всех задач

- [ ] **Прогнать оба набора тестов**

```bash
make test && make test-integration
```

- [ ] **Разовая чистка прода** (см. спеку §8)

```sql
-- Сначала посмотреть, потом удалять: ожидается ровно одна строка (78ec439d).
SELECT id, freq, byweekday, time_local, starts_on, ends_on FROM recurrence_rules r
WHERE NOT EXISTS (SELECT 1 FROM lessons l WHERE l.rule_id = r.id)
  AND NOT EXISTS (SELECT 1 FROM events  e WHERE e.rule_id = r.id);
```

Курс «Английский» (`1e1db4a8-be9d-43c0-ba17-0f564c30e690`) лечится из интерфейса: «Восстановить» → «Архивировать». Новая логика снимет 51 будущий урок и закроет правило.

- [ ] **Ручной прогон по чеклисту спеки §9**
