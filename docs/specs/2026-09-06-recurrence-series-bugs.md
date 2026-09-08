# Баги вокруг серий уроков (recurrence_rules)

**Дата:** 2026-09-06
**Статус:** к исправлению
**Область:** `repository/lesson.go`, `repository/recurrence.go`, `service/lesson.go`, `service/event.go`, `service/recurrence.go`, `service/course.go`, `router/router.go`, `frontend/src/app/(dashboard)/{calendar,courses}`, `frontend/src/components/lessons/LessonQuickPopover.tsx`

**Контекст.** После переезда с `lessons.series_id` на `recurrence_rules` (миграции 033, 036, 037 — см. `docs/specs/2026-08-31-calendar-events-and-scheduling-flow.md`, фаза 3) серверная механика серий реализована полностью: `PUT /lessons/:id?scope=one|following|all` и `DELETE /lessons/:id?scope=...` работают, `SplitRule`, `CloseRule`, `DeleteFutureByRule` и материализация на месте. Но до этой механики нельзя добраться из интерфейса, а одна из трёх областей разрушает данные.

Ниже пять проблем в том порядке, в котором их надо чинить.

---

## Баг 1. `rule_id` не селектится в списках уроков — весь серийный UI мёртв

**Что происходит.** На странице курса `/courses/:id` нельзя ни изменить время всей серии, ни удалить уроки начиная с какого-то. Диалог выбора области (`RecurrenceScopeDialog`) написан, подключён и не открывается никогда. Единственная доступная массовая операция — «Удалить все уроки».

**Где.** `repository/lesson.go` — три выборки не включают `rule_id` и `occurrence_date` ни в `SELECT`, ни в `Scan`:

- `GetByCourse` (строка 57)
- `GetByCoursePaged` (строка 77)
- `GetByPeriod` (строка 248)

`GetByIDForTutor` (строка 154) их селектит — поэтому серверная логика отрабатывает корректно, когда `scope` до неё доезжает.

**Механизм.** `models.Lesson.RuleID` всегда `nil` → в JSON поле отсутствует (`omitempty`) → на фронте `lesson.rule_id` всегда `undefined`. А обе точки входа в серийную правку гейтятся ровно на нём:

```
frontend/src/app/(dashboard)/courses/[id]/page.tsx:154   handleLessonSubmit → if (editingLesson.rule_id)
frontend/src/app/(dashboard)/courses/[id]/page.tsx:181   handleDeleteLesson → if (lesson.rule_id)
```

**Побочный эффект, портящий данные.** Правка уходит без `scope`, сервер подставляет `one`, и `service/lesson.go:246` вызывает `MarkOverride`. Каждый урок, которому руками меняли время, **навсегда выведен из-под правила**: `DeleteFutureByRule` фильтрует по `is_override = FALSE`, поэтому после починки «изменить все» такие уроки не тронет.

Найти пострадавшие: `SELECT * FROM lessons WHERE is_override;`

**Воспроизведение.** Создать еженедельную серию → `/courses/:id` → карандаш на любом уроке → изменить время → сохранить. Диалог области не появляется, меняется один урок, он получает `is_override = TRUE`.

**Куда чинить.** Добавить `rule_id, occurrence_date` в три запроса и соответствующие `Scan`. На фронте больше ничего не нужно — существующий UI оживёт сам. Заодно стоит завести общую константу колонок урока по образцу `eventCols` в `repository/event.go:15`, чтобы поля нельзя было забыть в четвёртом месте.

---

## Баг 2. `scope=all` уничтожает будущие вхождения вместо переноса

**Критично: чинить строго до бага 1.** Иначе оживший интерфейс даст пользователю кнопку, сносящую расписание.

**Что происходит.** «Изменить все» удаляет все будущие уроки серии и не создаёт их заново. Остаются: прошедшие уроки на старом времени, один отредактированный на новом и несколько штук на дальнем краю горизонта.

**Где.** `service/lesson.go:updateSeries` (ветка `scope == all`) и симметричная копия `service/event.go:applyToSeries` (строки 110–141). Корень — `repository/recurrence.go:UpdateTiming` (строка 117).

**Механизм.**

1. `UpdateRuleTiming` меняет только `time_local` и `duration_minutes`. `materialized_until` остаётся там, где стоял, — на горизонте (сегодня + 6 месяцев).
2. `DeleteFutureByRule` сносит все будущие вхождения правила со статусом `scheduled` и без `is_override`.
3. `Materialize` (`service/recurrence.go:115`) считает `Occurrences(rule, rule.MaterializedUntil, horizon)`. Так как `materialized_until` уже на горизонте, функция либо возвращает `0` сразу (строка 120), либо создаёт только тонкую полоску между старым и новым горизонтом.
4. Окно между «сегодня» и старым горизонтом остаётся пустым.

**Почему `following` при этом работает.** `repository/recurrence.go:Split` (строка 92) явно ставит новому правилу `materialized_until = $2::date`, поэтому материализация идёт от даты разреза. В `UpdateTiming` этого нет.

**Это уже живёт в проде — для событий.** Для уроков до `scope=all` не добраться (баг 1), но `EventQuickPopover` показывает кнопку «Все» в календаре. «Спортзал каждый понедельник» → «Изменить все» → будущие события удалены.

**Почему тесты зелёные.** `service/scope_test.go:154` (`TestLessonUpdate_ScopeAll`) работает на моках: фикстура правила создаётся без `MaterializedUntil` (нулевое время), а тест не проверяет, какие даты уходят в `InsertOccurrences`. Баг воспроизводится только на живой БД — нужен интеграционный тест (`make test-integration`) либо юнит-тест, проверяющий вызов `SetMaterializedUntil` с датой правимого вхождения.

**Куда чинить.** При `scope == all` после `UpdateRuleTiming` откатить границу материализации на `occurrence_date` правимого урока — тот же приём, что в `Split`. Детали:

- `SetMaterializedUntil` сейчас есть только на `RecurrenceRepository` (`repository/recurrence.go:124`), на `RecurrenceService` (`service/recurrence.go:18`) его нет — метод надо вынести в интерфейс.
- Обновить моки: `mockRecurrenceRepo` в `service/`. Рассинхрон интерфейса и моков ломает компиляцию тестов — главный footgun проекта.
- Починить **обе** копии логики: `service/lesson.go` и `service/event.go`.

---

## Баг 3. Правило переживает удаление своих уроков — они возвращаются

**Что происходит.** После «Удалить все уроки» курса через какое-то время в календаре снова появляются уроки этого курса — примерно через полгода от текущей даты, по одному-два в неделю.

**Где.** `repository/lesson.go:DeleteByCourse` (строка 183) — `DELETE FROM lessons WHERE course_id = ...`, таблица `recurrence_rules` не трогается.

**Механизм.** Правило без `ends_on` (такие создаются и из календаря через `SlotCreatePopover`, и из модалки онбординга) остаётся в выборке `DueForMaterialization`. Периодическая задача (`worker/run.go:106`, раз в сутки плюс `RunOnStart: true` на каждом редеплое) двигает горизонт, и `Materialize` создаёт вхождения в полоске между старым и новым горизонтом.

Найти это правило из интерфейса нельзя: единственный путь к правилу — через урок, а уроков не осталось.

**Важно про порядок операций.** Правило не связано с курсом напрямую — связь только через `lessons.rule_id`. Значит закрывать или удалять правила надо **до** удаления уроков, иначе связь потеряна:

```sql
SELECT DISTINCT rule_id FROM lessons WHERE course_id = $1 AND rule_id IS NOT NULL;
```

**Осиротевшие правила в текущих данных:**

```sql
SELECT r.id, r.time_local, r.byweekday, r.ends_on, r.materialized_until
FROM recurrence_rules r
WHERE NOT EXISTS (SELECT 1 FROM lessons l WHERE l.rule_id = r.id)
  AND NOT EXISTS (SELECT 1 FROM events  e WHERE e.rule_id = r.id);
```

---

## Баг 4. Архивация курса не убирает будущие уроки из календаря

**Что происходит.** Архивированная группа продолжает висеть в расписании, и для неё продолжают создаваться новые уроки.

**Где.**

- `service/course.go:78` (`courseService.Delete`) — это `UPDATE courses SET is_active = FALSE`, и больше ничего.
- `repository/lesson.go:GetCalendar` (строка 194) фильтрует только по `tutor_id`, без `is_active`.
- Правило курса остаётся живым — работает механизм из бага 3.

**Ожидаемое поведение задано текстом диалога** (`frontend/src/app/(dashboard)/courses/page.tsx:140`): «Архивировать курс? Завершённые уроки останутся в календаре» — то есть завершённые остаются, будущие исчезают.

**Куда чинить.** При архивации: закрыть правила курса (`SetEndsOn` на сегодня) и удалить будущие уроки со статусом `scheduled`. Фильтр `is_active` в `GetCalendar` **не добавлять** — он спрячет и завершённые уроки, что противоречит обещанию диалога.

Потребуется зависимость: `courseService` собран из `courseRepo` + `studentRepo` (`router/router.go:55`) и до уроков и правил не достаёт. Лучший вариант — узкий интерфейс к `lessonService` (он уже держит `recurrence` и `courseRepo`), с переносом строки создания ниже `lessonService` в `router.go:58`. Цикла зависимостей не возникает: `lessonService` зависит от `courseRepo`, а не от `courseService`.

Ту же операцию переиспользует баг 3 — это одна общая функция «отцепить и закрыть серии курса», вызываемая из двух мест.

---

## Баг 5. Попап урока в календаре не умеет ни времени, ни удаления, ни серии

**Что происходит.** Клик по уроку в календаре открывает попап, где есть только статус, заметки и посещаемость. Нельзя изменить время, нельзя удалить, нет признака, что урок принадлежит серии. У события рядом всё это есть.

**Где.**

- `frontend/src/app/(dashboard)/calendar/page.tsx:238` (`handleEventClick`) собирает объект `QuickLesson` и **выбрасывает `rule_id`**, хотя API его отдаёт (`repository/lesson.go:219`, поле `CalendarLesson.rule_id`).
- `frontend/src/components/lessons/LessonQuickPopover.tsx` — нет полей времени и длительности, нет кнопки удаления, `handleSave` отправляет `scheduled_at: lesson.scheduledAt` без изменений.

**Рабочий образец для переноса.** `frontend/src/components/calendar/EventQuickPopover.tsx:45-83`: признак `isSeries = !!event.rule_id`, отложенный вопрос об области через состояние `asking`, вызовы `save(scope)` / `remove(scope)`, внизу `RecurrenceScopeDialog`. Структуру можно переносить почти дословно.

**Почему у событий работает, а у уроков нет.** `repository/event.go:15` — константа `eventCols` включает `rule_id, occurrence_date` во все выборки событий. У уроков такой общей константы нет, и поля забыли в трёх местах (баг 1).

**Отдельно:** drag-and-drop должен и дальше применять `scope=one` без диалога — это осознанное решение спеки календаря, перетаскивание обязано оставаться безопасным жестом.

---

## Порядок работ

1. **Баг 2** — иначе всё остальное опасно.
2. **Баг 1** — три `SELECT`, после них серийный UI на странице курса работает.
3. **Баги 3 и 4** — общая правка: закрытие правил при удалении уроков и при архивации курса.
4. **Баг 5** — попап урока в календаре.

## Что проверить после

- Серия из 10 еженедельных уроков, «изменить все» на пятом → первые четыре на старом времени, с пятого по десятый на новом, ни одного потерянного, ни одного лишнего.
- То же для события — через `EventQuickPopover`.
- «Это и все следующие» на удаление → текущий урок отменён (`cancelled`, не удалён), последующие удалены, правило закрыто, ночная джоба их не возвращает.
- Вручную перенесённый урок (`is_override`) переживает обе операции.
- «Удалить все уроки» → через сутки (или после редеплоя, `RunOnStart`) уроки не появились.
- Архивированный курс → будущих уроков в календаре нет, завершённые на месте, новых не создаётся.
- `make test` и `make test-integration` — второй обязателен, баг 2 на моках не ловится.

## Обходной путь до починки

Поменять время всей серии можно вручную, используя ту же машинерию:

```sql
-- 1. найти правило
SELECT r.id, r.time_local, r.byweekday, c.subject, count(l.id) AS lessons
FROM recurrence_rules r
JOIN lessons l ON l.rule_id = r.id
JOIN courses c ON c.id = l.course_id
WHERE c.tutor_id = '<tutor_id>'
GROUP BY r.id, r.time_local, r.byweekday, c.subject;

-- 2. новое время + откат границы материализации на сегодня
UPDATE recurrence_rules
   SET time_local = '10:00', materialized_until = CURRENT_DATE
 WHERE id = '<rule_id>';

-- 3. снести будущие вхождения; проведённые, отменённые и вручную
--    перенесённые не трогаются
DELETE FROM lessons
 WHERE rule_id = '<rule_id>'
   AND occurrence_date > CURRENT_DATE
   AND is_override = FALSE
   AND status = 'scheduled';
```

Дальше уроки пересоздаст ночная джоба — или сразу редеплой (`RunOnStart: true`). Прошедшие уроки остаются на старом времени, что и правильно.
