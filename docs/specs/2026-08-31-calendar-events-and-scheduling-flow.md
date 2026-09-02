# Календарь как единая рабочая поверхность + переработка флоу планирования

**Дата:** 2026-08-31
**Статус:** Фазы 0–1 реализованы и смержены в `main`, фаза 2 следующая
**Область:** `migrations/`, `models/`, `repository/`, `service/`, `handlers/`, `router/`, `jobs/`, `worker/`, `frontend/src/app/(dashboard)/calendar`, `frontend/src/components/{lessons,tasks,events,calendar}`

> Спека разбита на фазы. **Реализуй по одной фазе за раз**, каждая фаза самодостаточна и деплоится отдельно. Не начинай следующую, пока предыдущая не смержена.

---

## 1. Проблема

### 1.1. Флоу планирования навязывает пользователю модель данных

Чтобы поставить первый повторяющийся урок, репетитор сейчас проходит три экрана и три модалки:

1. `/courses?tab=students` → «Добавить» → `StudentForm` (4 поля)
2. `/courses` → «Добавить» → `CourseForm` (7 полей, первое из которых — «Тип курса: индивидуальный / групповой»)
3. `/courses/:id` → «Добавить урок» → `LessonForm` (7 полей)

Итого ~14 кликов, 18 полей, три ментальных контекста. Конкретные дефекты:

- `frontend/src/app/(dashboard)/students/page.tsx` — редирект на `/courses?tab=students`. Ученик подчинён курсу; для репетитора первичен человек.
- `CourseForm` первым полем спрашивает «Тип курса». Это буквально выбор между `courses.student_id NOT NULL` и записью в `course_enrollments` — деталь реализации.
- `CreateCourseRequest.PricePerCycle` имеет `validate:"required,gt=0"` — нельзя завести бесплатный/пробный курс, и цена требуется до того, как назначен хоть один урок.
- `CreateCourseRequest.StartedAt` — `required` без дефолта.
- Выбор ученика в `CourseForm` — `<Select>` по всему списку `useStudents()`, без поиска и без создания на лету. Если ученика нет — тупик.
- `subject` — свободный `<Input>`. Каждый курс предмет перепечатывается, «Математика» и «математика» становятся разными значениями.
- Групповой курс нельзя создать вместе с учениками: только `POST /courses/:id/enrollments` по одному, bulk-эндпоинта нет.

### 1.2. Календарь умеет только уроки и задачи

`frontend/src/app/(dashboard)/calendar/page.tsx:handleSelect` на выделение пустого слота открывает `TaskCreatePopover` — то есть самый естественный жест в продукте создаёт **задачу**, а не урок. Пути «из календаря поставить урок» не существует вообще.

При этом репетитору нужны в календаре и личные события: врач, спортзал, пробный урок с лидом, подготовка материалов. Сейчас их некуда положить: `tasks` — это канбан со шкалой срочности (`not_urgent | urgent | very_urgent | done`) и намеренно необязательным временем (миграция `017`).

### 1.3. Повторения сломаны

`generateDates()` в `frontend/src/app/(dashboard)/courses/[id]/page.tsx` раскатывает даты на клиенте и шлёт массив в `POST /lessons/bulk`. Правило нигде не хранится, поэтому:

- «перенести все следующие» сделано костылём `UpdateSeriesRequest{from_date, new_time}`;
- продлить серию или исключить каникулы нечем;
- **баг:** `const limit = opts.count ?? 200`, а `endDate` равен `null`, если у курса нет `ended_at`. Подсказка в `LessonForm` обещает «создастся на 1 год вперёд», код создаёт 200 уроков (для еженедельного курса — почти 4 года строк);
- **баг:** `weekly_custom` с пустым списком дней молча возвращает `[base]` — один урок вместо серии, без ошибки валидации;
- времязависимость: `d.setDate(...)` по локальному времени + `.toISOString()`. Сейчас безопасно (Казахстан с 2024 — единый UTC+5 без перевода часов), сломается при выходе за пределы РК.

---

## 2. Продуктовый принцип

Календарь Амиды должен показывать **всю занятость репетитора**, а не только уроки. Смысл ровно один: глядя на неделю, видеть, когда ты занят и когда свободен, чтобы поставить туда урок или другое событие. Если врач и спортзал живут в другом календаре, картина занятости в Амиде неполная, планировать по ней нельзя, и репетитор уходит обратно в Google.

Отсюда следствие: **всякое событие занимает время.** Отдельного флага «занимает / не занимает» у события нет — событие в календаре и есть занятость. Занятость дают уроки в статусе `scheduled` и все события. Задачи — нет: задача это «сделать когда-нибудь», её слот условный и двигается свободно.

Второй принцип: **курс — производная сущность, а не действие пользователя.** Схема это уже позволяет (`courses.student_id` nullable с миграции `002`). Пользователь заводит ученика и ставит уроки; курс создаётся неявно по паре «ученик + предмет» и редактируется потом как строка «Математика — 5 000 ₸/урок» на карточке ученика.

## 3. Архитектурные решения

### 3.1. События — отдельная таблица `events`, не расширение `tasks`

**Решение:** новая сущность.

Почему не расширять `tasks`:

- `tasks.status` — шкала срочности канбана. У «спортзала» нет срочности; пришлось бы прятать поле для части строк, то есть заводить режим внутри одной модели.
- `tasks.scheduled_at` намеренно nullable (миграция `017`, канбан-карточки без слота). Событие всегда имеет время. Nullable-время у всегда-временнóй сущности — источник багов.
- `GetBoard` пришлось бы вечно фильтровать события из канбана.
- Задачам не нужны повторения; событиям нужны. Повторяющаяся канбан-карточка — бессмыслица.

Календарь становится лентой из трёх источников: `lessons`, `events`, `tasks` (только те, у которых заполнен `scheduled_at`).

### 3.2. Повторения — правило + материализация в скользящем горизонте

**Решение:** одна общая машинерия повторений для `lessons` и `events`. Правило хранится в `recurrence_rules`; вхождения — реальные строки в `lessons`/`events` со ссылкой `rule_id`.

Почему не чистая развёртка правила на лету:
`lessons.id` — внешний ключ для `lesson_attendances`, `lesson_tasks`, комнат LiveKit, досок и платежей. Вхождение обязано иметь стабильный ID. Плюс календарные запросы сейчас — обычные range-скан по индексам из миграции `005`, и это надо сохранить.

Почему не чистая материализация (как сейчас):
«Спортзал каждый понедельник» не имеет конца. Сегодняшний код молча создаёт 200 строк.

**Гибрид:** правило хранится, вхождения материализуются на горизонт **6 месяцев вперёд**; фоновая River-джоба (`jobs/jobs.go` + `worker/run.go` уже есть) еженочно дотягивает горизонт для правил, у которых `materialized_until < now() + 6 months`.

### 3.3. Google Calendar — вне области этой спеки

Двусторонняя синхронизация (OAuth, вебхуки, разрешение конфликтов) — отдельный проект. В схеме зарезервировать поля под неё (см. 5.1) и в фазе 5 сделать дешёвый односторонний ICS-фид.

---

## 4. Фаза 0 — багфиксы повторений

> **Реализовано** (коммит `aee49f8`). Раскатка дат вынесена из страницы курса в
> `frontend/src/lib/recurrence.ts` (`MAX_HORIZON_MONTHS`, `MAX_OCCURRENCES`,
> `generateDates`, `lessonsPlural`) — она нужна и форме для превью, и странице
> для сабмита; рядом `recurrence.test.ts` на `node:test`.

**Без миграций. Только `frontend/src/app/(dashboard)/courses/[id]/page.tsx` и `frontend/src/components/lessons/LessonForm.tsx`.**

1. В `generateDates()` ввести жёсткий предел по дате, а не только по числу: если `opts.count` не задан и `courseEndAt` пуст — генерировать не более чем на 12 месяцев вперёд от `base`. Константа `MAX_HORIZON_MONTHS = 12`, `MAX_OCCURRENCES = 200` оставить как второй предохранитель.
2. `weekly_custom` с пустым `days` — не возвращать `[base]`, а не давать отправить форму: в `LessonForm` заблокировать кнопку и показать ошибку «Выберите хотя бы один день недели», если `recType === 'weekly_custom' && recDays.length === 0`.
3. Перед отправкой показывать точное число: кнопка «Создать N уроков (до 15 июня 2027)» вместо «Создать уроки». Пользователь должен видеть, сколько строк он создаёт.

**Критерии приёмки**

- Курс без `ended_at`, еженедельный повтор, поле «количество» пустое → создаётся ≤ 53 урока, последний не позже чем через 12 месяцев.
- `weekly_custom` без выбранных дней → форма не отправляется, видна ошибка.
- Текст кнопки содержит фактическое число уроков и дату последнего.

---

## 5. Фаза 1 — сущность `events` и единая лента календаря

> **Реализовано полностью** (коммиты `373d0b6`, `c1c0c0a`, `b2ac186`, `055a724`,
> смержено в `main`): миграция, CRUD `/events`, лента, конфликты, десктопный
> календарь, `MobileWeekCalendar` на ленте, предупреждение о пересечении при
> drag-and-drop (`warnOnConflict`) и в `LessonForm`.

### 5.1. Миграция `031_events.sql`

```sql
-- +goose Up
CREATE TABLE events (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tutor_id         UUID        NOT NULL REFERENCES tutors(id) ON DELETE CASCADE,
    title            TEXT        NOT NULL,
    kind             TEXT        NOT NULL DEFAULT 'personal'
                                 CHECK (kind IN ('personal','work','trial')),
    starts_at        TIMESTAMPTZ NOT NULL,
    duration_minutes INT         NOT NULL CHECK (duration_minutes > 0),
    color            TEXT        NOT NULL DEFAULT '',
    location         TEXT        NOT NULL DEFAULT '',
    notes            TEXT        NOT NULL DEFAULT '',

    -- зарезервировано под внешнюю синхронизацию, в этой фазе не используется
    external_source  TEXT        NOT NULL DEFAULT '',
    external_id      TEXT        NOT NULL DEFAULT '',
    external_etag    TEXT        NOT NULL DEFAULT '',

    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_events_tutor_starts ON events(tutor_id, starts_at);

-- +goose Down
DROP TABLE IF EXISTS events;
```

Поля `busy` нет намеренно: любое событие занимает время (см. раздел 2).
Поля `all_day` нет по той же причине: событие «на весь день» не отвечает на
вопрос «когда я занят», а в сетке уезжает в отдельную полосу FullCalendar,
которая съедает высоту и выпадает из проверки занятости. Календарь рисуется с
`allDaySlot={false}`.

### 5.2. Что означает `kind`

| kind | что это | пример |
|---|---|---|
| `personal` | личное, не про работу | врач, спортзал, семья |
| `work` | работа, но не урок | подготовка материалов, созвон с родителем |
| `trial` | запланированный пробный урок | «Пробный — Асель» |

`kind` делает ровно две вещи и не влияет ни на занятость, ни на что-либо ещё:

1. Задаёт цвет и иконку по умолчанию (пользовательский `color` их перекрывает).
2. Даёт фильтр **«Показывать личные события»** — тумблер в шапке календаря, состояние в `localStorage`. Нужен, когда репетитор показывает расписание ученику или делает скриншот: «врач» на экране лишний.

`kind = 'trial'` дополнительно рисует в поповере события кнопку «Начать пробный урок» — см. раздел 9. Никакой карточки лида и отслеживания конверсии он не заводит.

### 5.3. Модель `models/event.go`

```go
package models

import "time"

type Event struct {
    ID              string    `json:"id"`
    TutorID         string    `json:"tutor_id"`
    Title           string    `json:"title"`
    Kind            string    `json:"kind"`
    StartsAt        time.Time `json:"starts_at"`
    DurationMinutes int       `json:"duration_minutes"`
    Color           string    `json:"color"`
    Location        string    `json:"location"`
    Notes           string    `json:"notes"`
}

type CreateEventRequest struct {
    Title           string    `json:"title"            validate:"required,max=200"`
    Kind            string    `json:"kind"             validate:"omitempty,oneof=personal work trial"`
    StartsAt        time.Time `json:"starts_at"        validate:"required"`
    DurationMinutes int       `json:"duration_minutes" validate:"required,gt=0"`
    Color           string    `json:"color"            validate:"omitempty,max=32"`
    Location        string    `json:"location"         validate:"omitempty,max=200"`
    Notes           string    `json:"notes"            validate:"omitempty,max=2000"`
}

// Правка события — полная замена тех же полей, отдельного типа заводить незачем.
type UpdateEventRequest = CreateEventRequest
```

### 5.4. Слои

По существующей схеме проекта:

- `repository/event.go` — `Create`, `GetByID`, `GetByRange(ctx, tutorID, from, to)`, `Update`, `Delete`,
  `GetOccupiedInRange(ctx, tutorID, from, to, excludeType string, excludeID *string)`. Исключение
  самого себя нужно при переносе: иначе элемент конфликтует с собственным старым слотом.
- `service/event.go` — проверка принадлежности тьютору, дефолт `Kind = "personal"`.
- `handlers/event.go` — стандартный набор, `tutorID` из контекста как везде.
- `router/router.go`:

```go
auth.GET   ("/events",     eventHandler.GetByRange)
auth.POST  ("/events",     eventHandler.Create)
auth.GET   ("/events/:id", eventHandler.GetByID)
auth.PUT   ("/events/:id", eventHandler.Update)
auth.DELETE("/events/:id", eventHandler.Delete)
```

### 5.5. Единая лента календаря

Заменить два запроса страницы календаря (`useCalendar` + `useTasks`) одним.

`GET /calendar/feed?from=&to=&kinds=lesson,event,task` →

```json
{
  "items": [
    { "type": "lesson", "id": "...", "title": "Математика — Айгерим", "starts_at": "...",
      "duration_minutes": 60,
      "lesson": { "course_id": "...", "status": "scheduled", "is_group": false,
                  "cycle_position": 3, "cycle_size": 8, "paid": true } },
    { "type": "event", "id": "...", "title": "Спортзал", "starts_at": "...",
      "duration_minutes": 90,
      "event": { "kind": "personal", "color": "", "location": "" } },
    { "type": "task", "id": "...", "title": "Проверить ДЗ", "starts_at": "...",
      "duration_minutes": 30,
      "task": { "status": "urgent" } }
  ]
}
```

Общие поля (`type`, `id`, `title`, `starts_at`, `duration_minutes`) — наверху, специфика во вложенном объекте по имени типа. Фронт рисует сетку по общим полям и лезет внутрь только для бейджей и поповеров.

`kinds` — опциональный фильтр, по умолчанию все три.

Реализация: `service/calendar.go`, который параллельно (`errgroup`) дёргает три источника и сливает результат, отсортированный по `starts_at`. Зависимости объявлены узкими интерфейсами на один метод (`lessonFeedSource` и т.д.), а не целыми сервисами: и связность меньше, и мок в тесте короче.

Старый `GET /calendar` остаётся **навсегда**, это не переходный костыль. Два эндпоинта живут по назначению:

| эндпоинт | что отдаёт | кто читает |
|---|---|---|
| `GET /calendar` | только уроки (`CalendarLesson[]`) | дашборд и мини-календарь в сайдбаре — события и задачи им там только мешают |
| `GET /calendar/feed` | единая лента из трёх источников | страница календаря |

На фронте это две функции в `useCalendar.ts`: `useCalendar()` и `useCalendarFeed()`. Ключ кэша у обеих начинается с `['calendar']`, поэтому одна инвалидация чинит оба.

### 5.6. Проверка занятости

`GET /calendar/conflicts?starts_at=&duration_minutes=&exclude_type=&exclude_id=`

Возвращает `{ "conflicts": [ {type, id, title, starts_at, duration_minutes} ] }` — всё, что пересекается по времени:

- уроки в статусе `scheduled`;
- все события.

Задачи не считаются занятостью и в ответ не попадают.

Пересечение: `starts_at < $end AND starts_at + duration * interval '1 minute' > $start`.

Используется в трёх местах: при создании урока, при создании события и при drag-and-drop в календаре. **Конфликт не блокирует сохранение** — показывается предупреждение «Пересекается со „Спортзал" 17:00–18:30» с кнопками «Всё равно создать» / «Отмена». Жёсткая блокировка раздражает: репетитор сам знает, когда наложение осознанное.

В `SlotCreatePopover` (создан уже в фазе 1, см. 5.7) проверка вызывается сразу при открытии поповера, ещё до заполнения полей: репетитор видит «Занято: Спортзал» в момент выбора слота, а не после сабмита. То же предупреждение — в `LessonForm`.

### 5.7. Фронт

- `frontend/src/types/api.ts` — типы `CalendarItem` (размеченное объединение по `type`), `Event`, `EventKind`.
- `frontend/src/lib/api/events.ts`, `frontend/src/lib/hooks/useEvents.ts` по образцу `useTasks`. Отдельного кэша под события нет: они видны только в ленте, поэтому мутации просто инвалидируют `['calendar']`. Там же `useConflicts()` — запрос из 5.6.
- `frontend/src/lib/api/calendar.ts` — рядом с `list()` появляются `feed()` и `conflicts()`; в `frontend/src/lib/hooks/useCalendar.ts` — `useCalendarFeed()` рядом с `useCalendar()` (см. таблицу в 5.5).
- `frontend/src/lib/eventKind.ts` — подписи `kind`, сопоставление `kind` → цвет и `formatTimeRange()` для строки «Занято: … 17:00 – 18:30».
- `frontend/src/components/calendar/SlotCreatePopover.tsx` — заменяет удалённый `TaskCreatePopover`; в этой фазе два таба, «Событие» и «Задача», плюс строка занятости под шапкой. Таб «Урок» добавляет фаза 2 (6.1).
- `frontend/src/components/calendar/EventQuickPopover.tsx` — правка и удаление события по клику на блок.
- `calendar/page.tsx` — строить события FullCalendar из одной ленты, `allDaySlot={false}`. Цвета уроков (`FC_COLORS`), задач и событий — CSS-переменные `--cal-*` в `globals.css`, чтобы тёмная тема правилась в одном месте; событию цвет выбирает `kind`, непустой `event.color` перекрывает.
- Тумблер «Показывать личные события» — `customButton` в тулбаре FullCalendar, а не отдельная строка над сеткой (строка съедала высоту и висела без хозяина). Состояние в `localStorage` (`tg_cal_show_personal`), по умолчанию включён. Скрывает только `kind = 'personal'`.

**Критерии приёмки**

- Событие создаётся, видно в календаре, переносится drag-and-drop, редактируется, удаляется.
- Календарь делает один сетевой запрос на диапазон вместо двух.
- Постановка урока на время, занятое событием, показывает предупреждение и позволяет продолжить.
- Тумблер прячет личные события и не трогает остальные.

## 6. Фаза 2 — единая точка создания в календаре

Здесь чинится главный дефект UX: клик по пустому слоту.

### 6.1. `SlotCreatePopover`

Компонент уже стоит в `calendar/page.tsx:handleSelect` с фазы 1, но умеет только «Событие» и «Задача», дефолт — «Событие». Фаза 2 добавляет третий таб и переносит дефолт: **«Урок»**.

```
┌──────────────────────────────────────┐
│ Вт, 2 сент · 17:00 – 18:00        ✕ │
│ ┌──────┬─────────┬────────┐          │
│ │ Урок │ Событие │ Задача │          │
│ └──────┴─────────┴────────┘          │
│                                      │
│ [ Кто?                          ▾ ]  │  ← комбобокс учеников
│ [ Предмет                       ▾ ]  │  ← появляется после выбора ученика
│ 60 мин   ☐ Повторять по вторникам    │
│                                      │
│              [Отмена]  [Создать]     │
└──────────────────────────────────────┘
```

Если слот пересекается с уроком или событием, под шапкой поповера сразу
показывается строка «⚠ Занято: Спортзал 17:00–18:30» (запрос из 5.6).
Создать всё равно можно.

- **Урок** (дефолт): комбобокс учеников → предмет → длительность → чекбокс повтора.
- **Событие**: заголовок → категория (`personal` / `work` / `trial`, дефолт `personal`) → чекбокс повтора.
- **Задача**: заголовок, как сейчас.

Тип запоминается в `localStorage` в пределах сессии? **Нет.** Всегда сбрасывать на «Урок» — иначе репетитор, один раз создавший задачу, будет потом молча создавать задачи вместо уроков.

### 6.2. Комбобокс ученика с созданием на лету

Новый компонент `frontend/src/components/students/StudentCombobox.tsx`.

- Поиск по подстроке имени и фамилии, клиентская фильтрация по `useStudents()` (список тьютора помещается в память; на > 200 учеников перейти на серверный `?search=`).
- Если ничего не найдено — первая строка списка: **«Создать „Айгерим"»**. Клик создаёт ученика через `POST /students` только с `first_name` и сразу выбирает его. Остальные поля заполняются потом на карточке.
- Используется в трёх местах: `SlotCreatePopover`, `CourseForm`, диалог записи в группу.

### 6.3. Комбобокс предмета

Новый эндпоинт `GET /courses/subjects` → `["Математика", "Физика"]`:

```sql
SELECT DISTINCT subject FROM courses
WHERE tutor_id = $1 AND deleted_at IS NULL
ORDER BY subject
```

Компонент `SubjectCombobox` — те же правила, что у ученика: поиск + «Создать „Химия"» (просто принимает введённую строку). Заменяет `<Input placeholder="Математика">` в `CourseForm`.

При выборе ученика в `SlotCreatePopover` предмет предзаполняется предметом его последнего активного курса.

### 6.4. Неявное создание курса

`POST /lessons` расширить: принимать **либо** `course_id`, **либо** пару `student_id` + `subject`.

```go
type CreateLessonRequest struct {
    CourseID        string    `json:"course_id"        validate:"omitempty,uuid"`
    StudentID       string    `json:"student_id"       validate:"omitempty,uuid"`
    Subject         string    `json:"subject"          validate:"omitempty,min=2"`
    ScheduledAt     time.Time `json:"scheduled_at"     validate:"required"`
    DurationMinutes int       `json:"duration_minutes" validate:"required,gt=0"`
    Notes           string    `json:"notes"            validate:"omitempty,max=500"`
}
```

Валидация на уровне сервиса: заполнено либо `CourseID`, либо оба `StudentID` и `Subject`; иначе `400`.

В `service/lesson.go` — get-or-create в одной транзакции:

```
найти активный курс по (tutor_id, student_id, subject, is_active)
если нет — создать курс с дефолтами:
    price_per_cycle   = самая частая цена этого тьютора, иначе 0
    lessons_per_cycle = самое частое значение, иначе 1
    started_at        = дата урока
создать урок в этом курсе
```

**Миграция `032_courses_student_subject_unique.sql` обязательна.** Транзакция от гонки
здесь не спасает: под `READ COMMITTED` два параллельных запроса (двойной клик, две
вкладки) оба увидят пустой `SELECT` до чужого `INSERT` и создадут два одинаковых курса.
Уникальности на эту тройку в схеме сейчас нет — значит, её надо завести:

```sql
CREATE UNIQUE INDEX idx_courses_tutor_student_subject ON courses(tutor_id, student_id, subject)
    WHERE is_active AND student_id IS NOT NULL;
```

Индекс частичный по двум причинам: у группового курса `student_id IS NULL` и одинаковых
групп по одному предмету может быть сколько угодно, а архивный курс (`is_active = FALSE`)
не должен мешать завести новый с тем же предметом.

> Soft-delete курса в схеме сделан через `is_active` (миграция `013`), колонки
> `deleted_at` у `courses` нет — все условия ниже читать соответственно.

Вставка курса — `INSERT ... ON CONFLICT DO NOTHING` с повторным `SELECT`: проигравший
гонку получает ноль строк и читает чужой курс вместо ошибки.

Перед накаткой на прод проверить, что дублей нет (`GROUP BY tutor_id, student_id, subject
HAVING count(*) > 1` по живым индивидуальным курсам) — иначе `CREATE UNIQUE INDEX` упадёт
на существующих данных.

Снять `gt=0` с `price_per_cycle` в `CreateCourseRequest` (заменить на `gte=0`), иначе неявное создание с нулевой ценой невозможно. Дать `started_at` дефолт «сегодня», если не передан.

То же самое для `POST /lessons/bulk`.

### 6.5. Ученик — самостоятельный раздел

- Удалить редирект в `frontend/src/app/(dashboard)/students/page.tsx`, сделать полноценную страницу списка.
- Добавить «Ученики» в `Sidebar.tsx` и `MobileBottomNav.tsx` отдельным пунктом **выше** «Курсы».
- Из `/courses` убрать вкладку «Ученики»; вкладки остаются «Активные» / «Архив».
- На карточке ученика (`students/[id]/page.tsx`) курсы показывать не списком-ссылкой, а строками с инлайн-редактированием цены: «Математика — 5 000 ₸/урок · 8 уроков в цикле».

### 6.6. Убрать вопрос про тип курса

- Из `CourseForm` убрать переключатель «Индивидуальный / Групповой». Форма всегда индивидуальная.
- Группа создаётся отдельным действием «Создать группу» на странице курсов, с полем «Ученики» — мультиселект на том же `StudentCombobox`.
- Новый эндпоинт `POST /courses/:id/enrollments/bulk` c телом `{ "student_ids": ["...", "..."] }`, чтобы группа собиралась одним сабмитом.

**Критерии приёмки**

- Из пустого календаря новый ученик и первый урок создаются за ≤ 6 взаимодействий, без перехода на другие экраны.
- Клик по пустому слоту по умолчанию предлагает урок.
- Курс ни разу не создаётся явным действием пользователя в основном сценарии.
- Ученик существует как раздел навигации.

---

## 7. Фаза 3 — общая машинерия повторений

### 7.1. Миграция `033_recurrence.sql`

```sql
-- +goose Up
CREATE TABLE recurrence_rules (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tutor_id           UUID        NOT NULL REFERENCES tutors(id) ON DELETE CASCADE,
    freq               TEXT        NOT NULL CHECK (freq IN ('daily','weekly','monthly')),
    interval_n         INT         NOT NULL DEFAULT 1 CHECK (interval_n > 0),
    byweekday          SMALLINT[]  NOT NULL DEFAULT '{}',   -- ISO 1=Пн … 7=Вс
    time_local         TIME        NOT NULL,
    tz                 TEXT        NOT NULL,                -- IANA, напр. 'Asia/Almaty'
    duration_minutes   INT         NOT NULL CHECK (duration_minutes > 0),
    starts_on          DATE        NOT NULL,
    ends_on            DATE,                                -- NULL = бессрочно
    max_count          INT,                                 -- NULL = без ограничения
    materialized_until DATE        NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_rules_materialize ON recurrence_rules(materialized_until)
    WHERE ends_on IS NULL OR ends_on > CURRENT_DATE;

ALTER TABLE lessons
    ADD COLUMN rule_id         UUID REFERENCES recurrence_rules(id) ON DELETE SET NULL,
    ADD COLUMN occurrence_date DATE,
    ADD COLUMN is_override     BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE events
    ADD COLUMN rule_id         UUID REFERENCES recurrence_rules(id) ON DELETE CASCADE,
    ADD COLUMN occurrence_date DATE,
    ADD COLUMN is_override     BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX idx_lessons_rule ON lessons(rule_id) WHERE rule_id IS NOT NULL;
CREATE INDEX idx_events_rule  ON events(rule_id)  WHERE rule_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_events_rule;
DROP INDEX IF EXISTS idx_lessons_rule;
ALTER TABLE events  DROP COLUMN is_override, DROP COLUMN occurrence_date, DROP COLUMN rule_id;
ALTER TABLE lessons DROP COLUMN is_override, DROP COLUMN occurrence_date, DROP COLUMN rule_id;
DROP TABLE IF EXISTS recurrence_rules;
```

Существующий `lessons.series_id` не трогать в этой миграции — он остаётся для уже созданных серий. Отдельным шагом (7.5) данные переезжают на `rule_id`, после чего `series_id` и эндпоинты `/lessons/series/:seriesId` удаляются.

### 7.2. Генератор — `service/recurrence.go`

```go
// Occurrences возвращает моменты начала вхождений правила в полуинтервале [from, to).
// Время считается как локальные стенные часы rule.TimeLocal в зоне rule.TZ
// и конвертируется в UTC для каждой даты отдельно — это и есть защита от перевода часов.
func Occurrences(rule models.RecurrenceRule, from, to time.Time) ([]time.Time, error)
```

Реализация: `time.LoadLocation(rule.TZ)`, затем для каждой подходящей календарной даты `time.Date(y, m, d, hh, mm, 0, 0, loc)`. Не складывать длительности в UTC — только пересборка по календарной дате.

Правила `freq`:
- `weekly` + `byweekday` — все перечисленные дни каждые `interval_n` недель, отсчёт недель от недели `starts_on` (понедельник).
- `weekly` с пустым `byweekday` — день недели берётся из `starts_on`.
- `daily` — каждые `interval_n` дней.
- `monthly` — то же число месяца, что у `starts_on`; если числа в месяце нет (31-е в феврале) — месяц пропускается.

Ограничители: остановиться на `ends_on`, на `max_count` вхождений от `starts_on`, либо на `to`.

### 7.3. Материализация

`service/recurrence.go`:

```go
// Materialize добивает вхождения правила до horizon, не трогая строки с is_override = true
// и не создавая дубликатов (уникальность по (rule_id, occurrence_date)).
func (s *recurrenceService) Materialize(ctx context.Context, ruleID string, horizon time.Time) (int, error)
```

Добавить частичный уникальный индекс, чтобы повторный запуск джобы был идемпотентен:

```sql
CREATE UNIQUE INDEX idx_lessons_rule_occurrence ON lessons(rule_id, occurrence_date)
    WHERE rule_id IS NOT NULL;
CREATE UNIQUE INDEX idx_events_rule_occurrence ON events(rule_id, occurrence_date)
    WHERE rule_id IS NOT NULL;
```

Вставка — `INSERT ... ON CONFLICT DO NOTHING`.

### 7.4. River-джоба

`jobs/jobs.go`:

```go
type RecurrenceExtendArgs struct{}
func (RecurrenceExtendArgs) Kind() string { return "recurrence_extend" }
func (RecurrenceExtendArgs) InsertOpts() river.InsertOpts {
    return river.InsertOpts{MaxAttempts: 3}
}
```

Воркер `worker/recurrence_extend.go`: раз в сутки выбирает правила с `materialized_until < CURRENT_DATE + INTERVAL '6 months'` и активные (`ends_on IS NULL OR ends_on > CURRENT_DATE`), для каждого вызывает `Materialize` и двигает `materialized_until`. Периодическую вставку джобы повесить на River periodic jobs в `worker/run.go`.

Горизонт: константа `RecurrenceHorizon = 6 * 30 * 24 * time.Hour` в `config`.

### 7.5. Семантика редактирования

Любое изменение вхождения, у которого `rule_id IS NOT NULL`, спрашивает область через диалог с тремя кнопками (компонент `frontend/src/components/calendar/RecurrenceScopeDialog.tsx`):

| Выбор | Что делает бэкенд |
|---|---|
| **Только это** | обновляет одну строку, ставит `is_override = TRUE` |
| **Это и все следующие** | закрывает старое правило `ends_on = occurrence_date - 1 day`; создаёт новое правило с новыми параметрами и `starts_on = occurrence_date`; удаляет будущие вхождения старого правила, **у которых `is_override = FALSE`**; материализует новое |
| **Все** | обновляет правило; пересоздаёт все будущие вхождения с `is_override = FALSE` |

`is_override` — это то, что не даёт «изменить все следующие» затереть вручную перенесённый урок. Перенос drag-and-drop одного вхождения **всегда** трактуется как «только это» без диалога — иначе перетаскивание становится опасным.

Удаление — та же тройка. «Только это» на вхождении правила ставит `status = 'cancelled'` для урока и удаляет строку для события.

API:

```
PATCH  /lessons/:id?scope=one|following|all
DELETE /lessons/:id?scope=one|following|all
PATCH  /events/:id?scope=one|following|all
DELETE /events/:id?scope=one|following|all
```

`scope` по умолчанию `one`.

### 7.6. Перенос существующих серий

Отдельная одноразовая миграция данных `034_series_to_rules.sql`: для каждого различного `lessons.series_id` восстановить правило по фактическим строкам (частота = медианный интервал между соседними уроками, `time_local` = время первого урока в `Asia/Almaty`, `starts_on` = дата первого, `ends_on` = дата последнего, `materialized_until` = дата последнего), проставить `rule_id` и `occurrence_date`, оставить `is_override = FALSE`.

После проверки на проде — миграция `035`, удаляющая `lessons.series_id`, `UpdateSeriesRequest`, `DELETE /lessons/series/:seriesId`, `PATCH /lessons/series/:seriesId` и клиентский `generateDates()`.

**Критерии приёмки**

- «Спортзал каждый понедельник» без даты окончания создаёт вхождения на 6 месяцев и не больше; через месяц джоба дотягивает горизонт.
- Перенос одного вхождения drag-and-drop не спрашивает область и не влияет на остальные.
- «Это и все следующие» с изменением времени не сдвигает вручную перенесённое вхождение в будущем.
- Правило с `tz = 'Europe/Berlin'` даёт вхождения с одинаковым локальным временем по обе стороны перевода часов.

---

## 8. Фаза 4 — быстрый онбординг ученика

### 8.1. Транзакционный эндпоинт

`POST /onboarding/student`

```json
{
  "first_name": "Айгерим",
  "phone": "+77001234567",
  "subject": "Математика",
  "price_per_cycle": 5000,
  "lessons_per_cycle": 1,
  "schedule": {
    "byweekday": [1, 3],
    "time_local": "17:00",
    "tz": "Asia/Almaty",
    "duration_minutes": 60,
    "starts_on": "2026-09-02",
    "ends_on": null
  }
}
```

Обязательно только `first_name`. Всё остальное опционально: без `subject` создаётся только ученик, без `schedule` — ученик и курс без уроков.

В одной транзакции: `students` → `courses` → `recurrence_rules` → материализация вхождений. Ответ — `{ student, course, lessons_created }`.

### 8.2. Модалка «Новый ученик»

Одна форма, одно обязательное поле, остальное со свёрнутыми дефолтами:

```
Имя*             [Айгерим                    ]
Телефон          [+7                         ]
─────────────────────────────────────────────
Предмет          [Математика               ▾ ]
Когда            [Пн][Вт][Ср][Чт][Пт][Сб][Вс]
                 17:00      60 мин
Оплата           5 000 ₸ за 1 урок    ⌄
─────────────────────────────────────────────
                        [Отмена]  [Добавить]
```

Дефолты: предмет — самый частый у тьютора; дни — сегодняшний день недели; время — 17:00; длительность — самая частая; цена — самая частая.

Точки входа: кнопка «Добавить ученика» на `/students`, пустое состояние дашборда, пустое состояние календаря.

**Критерии приёмки**

- Ученик + курс + серия на семестр создаются одним сабмитом за ≤ 30 секунд.
- Форма отправляется при заполненном только имени.
- Ошибка на любом шаге не оставляет частично созданных данных.

---

## 9. Фаза 5 — пробный урок в расписании и ICS

### 9.1. Что уже работает и не меняется

Пробный урок реализован и переделывать его не надо:

- `/trial` → `POST /calls/quick` создаёт комнату LiveKit, токен тьютора живёт 3 часа;
- гость заходит по публичной ссылке без регистрации и авторизации — `GET /public/quick/:id/guest-token`, имя вводит в форме входа;
- доска у всех пробных общая: `boards.course_id IS NULL` с уникальным индексом на тьютора (миграция `026`), нарисованное остаётся к следующему разу;
- после урока преподаватель заводит ученика и жмёт «Пригласить в кабинет» — `POST /students/:id/invite` выдаёт ссылку `/student/invite/<token>`, по которой ученик задаёт логин и пароль через `POST /student/auth/accept-invite` и забирает аккаунт.

### 9.2. Чего не хватает: запланированный пробный

Сейчас `/trial` умеет только «начать сейчас», и страница прямо пишет, что пробный не попадает в расписание. Значит договорённость «пробный в четверг в 18:00» нигде не видна, и слот выглядит свободным.

С появлением событий это закрывается без новой логики на бэкенде:

1. Репетитор создаёт событие с `kind = 'trial'`, заголовок «Пробный — Асель», четверг 18:00.
2. Слот занят, конфликты работают, событие видно в календаре.
3. В поповере события с `kind = 'trial'` — кнопка **«Начать пробный урок»**, которая дёргает существующий `POST /calls/quick` и ведёт в `/room/:id` ровно так же, как страница `/trial`.

Всё, что требуется: вынести обработчик из `frontend/src/app/(dashboard)/trial/page.tsx` в переиспользуемый хук `useStartQuickRoom()` и вызвать его из поповера события.

**Не делаем:** карточку лида, поля `lead_name` / `lead_phone`, счётчик конверсии пробных. Пробный — это слот в расписании, а не запись в CRM.

### 9.3. Известный риск (вне области спеки, но записан)

`handlers/call.go` держит активные пробные комнаты в памяти процесса — `quickRooms map[string]*quickRoom` с вытеснением через 3 часа. При редеплое на Railway посреди пробного урока `GET /public/quick/:id/guest-token` начнёт отдавать `404 room not found or ended`: преподаватель в комнате останется (его токен уже выдан), а потенциальный ученик войти не сможет. Это худший момент для сбоя из возможных. Лечится переносом состояния в таблицу или Redis — отдельной задачей.

### 9.4. ICS-фид

`GET /ics/:token` — публичный, без авторизации, токен на тьютора (хранить в `tutors.ics_token`, генерировать по запросу, давать возможность отозвать).

Отдаёт `text/calendar` со всеми уроками и событиями на ±6 месяцев. Репетитор подписывается на URL в Google Календаре и видит расписание в телефоне без всякого OAuth.

Односторонний, только чтение. Двусторонняя синхронизация — отдельная спека.

## 10. Что сознательно НЕ делаем

- Двустороннюю синхронизацию с Google Calendar. Поля в схеме зарезервированы, реализации нет.
- Разбор естественного языка в поле ввода («спортзал пн ср 7:00»). Заманчиво, но требует отдельной работы над качеством и локалью.
- Приглашения и участников событий. У репетитора нет коллег в системе.
- Флаг «занимает / не занимает время» у события. Все события занимают; отдельная настройка только запутает.
- Карточку лида и воронку по пробным урокам. Пробный — слот в расписании, а не CRM.
- Часовые пояса на уровне ученика. Всё в зоне тьютора; хранение `tz` в правиле — задел, а не фича.
- Напоминания и пуши. Отдельная тема, зависит от канала доставки.

---

## 11. Порядок работ

| Фаза | Содержание | Миграции | Оценка | Статус |
|---|---|---|---|---|
| 0 | Багфиксы повторений | нет | полдня | ✅ `aee49f8` |
| 1 | `events`, единая лента, конфликты | 031 | 2–3 дня | ✅ `373d0b6`, `c1c0c0a`, `b2ac186`, `055a724` |
| 2 | `SlotCreatePopover`, комбобоксы, неявный курс, раздел «Ученики» | 032 | 3–4 дня | ✅ `bff8ff5`, `7e56de7` (ветка `feat/implicit-course`, 032 на проде); ждёт ручного smoke |
| 3 | `recurrence_rules`, материализация, джоба, семантика правок | 033–035 | 4–5 дней | |
| 4 | `POST /onboarding/student`, модалка | нет | 2 дня | |
| 5 | Кнопка «Начать пробный» на событии, ICS | 036 | 1–2 дня | |

Всё сделанное — в ветке `feat/recurrence-fixes`.

Фазы 1 и 2 дают основной эффект для активации и идут в таком порядке. Фаза 3 — самая объёмная, но без неё «спортзал каждый понедельник» работать не будет.
