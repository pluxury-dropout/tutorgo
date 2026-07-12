# Кабинет ученика v2: задачи, цикл оплаченных, вход на доску

**Дата:** 2026-07-12
**Статус:** утверждён (дизайн)

Три независимые доработки кабинета ученика поверх смерженной v1
([[project_student_accounts]]). Одна спека, три раздела; план разобьёт на фазы.
Фичи 2 и 3 — почти чистый frontend, фича 1 — полный слой.

Базовый контекст:
- Ученик = запись в `students` c auth-колонками; вход по телефону/username.
- Student-роуты под `middleware.AuthStudent` (JWT `role:"student"`), без
  subscription-гейта. Authz к уроку — `EnrolledInLesson` (fails-closed, оба пути:
  `courses.student_id` И `course_enrollments`).
- Главная ученика — `/student/lessons` (туда редиректит логин); карточки уроков
  с кнопкой звонка.

---

## 1. Задачи (ДЗ)

Репетитор ставит задачу к конкретному уроку; ученик отмечает выполнение.
Двухстатусная модель (`done` bool) — без «проверки репетитором».

### Данные

Новая таблица `lesson_tasks`:

```sql
CREATE TABLE lesson_tasks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lesson_id   UUID NOT NULL REFERENCES lessons(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    done        BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX lesson_tasks_lesson_id_idx ON lesson_tasks(lesson_id);
```

Владение — через урок. Отдельный `tutor_id` не храним: урок уже tutor-scoped,
authz идёт через владение уроком (репетитор) или enrollment (ученик).

### Модель (`models/`)

```go
type LessonTask struct {
    ID          string    `json:"id"`
    LessonID    string    `json:"lesson_id"`
    Title       string    `json:"title"`
    Description string    `json:"description"`
    Done        bool      `json:"done"`
    CreatedAt   time.Time `json:"created_at"`
}

type CreateTaskRequest struct {
    Title       string `json:"title"       validate:"required,min=1,max=200"`
    Description string `json:"description" validate:"omitempty,max=1000"`
}

type UpdateTaskRequest struct {  // репетитор правит текст
    Title       string `json:"title"       validate:"required,min=1,max=200"`
    Description string `json:"description" validate:"omitempty,max=1000"`
}

type SetTaskDoneRequest struct { // ученик меняет только статус
    Done bool `json:"done"`
}
```

### Backend — репетитор (`middleware.Auth`)

Все проверяют, что урок задачи принадлежит `tutorID` (JOIN lessons→courses→
tutor_id, как в существующих lesson-запросах).

- `GET    /lessons/:id/tasks`      — список задач урока
- `POST   /lessons/:id/tasks`      — создать (`CreateTaskRequest`)
- `PUT    /tasks/:id`              — правка текста (`UpdateTaskRequest`)
- `DELETE /tasks/:id`              — удалить

### Backend — ученик (`middleware.AuthStudent`)

- `GET   /student/lessons/:id/tasks` — список; гейт `EnrolledInLesson(studentID, lessonID)`, иначе 403
- `PATCH /student/tasks/:id`         — `SetTaskDoneRequest`; ученик меняет **только** `done` и **только** у задач урока, в котором он enrolled

Проверка прав на `PATCH`: по `task.id` находим `lesson_id`, затем
`EnrolledInLesson(studentID, lesson_id)`. Fails-closed.

### Frontend

- **Репетитор:** блок «Задачи» в форме редактирования урока — список +
  добавить/править/удалить. Следует существующему паттерну lesson-формы.
- **Ученик:** на карточке урока (`/student/lessons`) — список задач с чекбоксом;
  тап по чекбоксу шлёт `PATCH .../done`. Оптимистичное обновление, откат при
  ошибке.

### Тесты (service-layer, testify/mock)

- create/update/delete под правильным tutorID; чужой урок → not found.
- student list/patch: enrolled → ok; не enrolled → forbidden; patch чужой
  задачи → forbidden.

---

## 2. Цикл оплаченных: остаток + бейджи

Позиция/размер цикла уже считаются от платежей
(`service/student.go:ListLessons` → `cyclePositionFromRank`). Ключ:
`cyclePositionFromRank` возвращает `position=0`, когда `rank` урока выходит за
сумму `lessons_count` всех платежей курса → урок **не оплачен**.

### Backend (минимум)

Добавить `Paid *bool` в `models.CalendarLesson`. В `ListLessons`, в том же цикле,
где уже есть `paymentsMap` и вызов `cyclePositionFromRank`:

- для урока с `Rank != nil`: `paid := pos > 0`, `lessons[i].Paid = &paid`.
- для урока без ранга/без платежей курса: `Paid` остаётся `nil` (нет данных о
  цикле — бейдж не показываем).

**Ноль новых запросов** — переиспользуем уже загруженные платежи.

### Frontend

- **Бейдж на карточке урока:** `paid === true` → «Оплачен», `paid === false` →
  «Не оплачен», `paid == null` → без бейджа.
- **Сводка по текущему циклу** вверху списка: «Оплачено N из M, осталось K».
  Выводится на клиенте из `cycle_position`/`cycle_size` ближайшего предстоящего
  урока (M = `cycle_size`, N = `cycle_position`, K = M − N).
  - *Fallback:* если клиентский расчёт окажется неудобным, переиспользовать
    существующий `GetCurrentCycles` (`CurrentCycleInfo`: Progress/CycleSize/
    LastAt, уже есть для дашборда репетитора), заскоупив на курсы ученика через
    отдельный `GET /student/cycles`. По умолчанию — клиентский расчёт (YAGNI).

### Тесты

- `ListLessons`: урок в пределах оплаты → `Paid=true`; урок за пределом →
  `Paid=false`; курс без платежей → `Paid=nil`. Проверяется на существующем
  student-service тесте с моками платежей.

---

## 3. Вход на доску с главной

Backend готов: `GET /student/lessons/:id/board-token` (`handlers/whiteboard.go`)
проверяет `EnrolledInLesson`, отдаёт `{invite_token, page_id}`.

### Frontend

Кнопка «Доска» на карточке урока в `/student/lessons`, рядом с «Войти в звонок»:

1. `GET /student/lessons/:id/board-token` → `{invite_token}`
2. `router.push('/board/join/' + invite_token)` — переиспользуем существующую
   гостевую страницу доски (`/board/join/[token]`).

Ноль нового board-UI.

### Проверить на smoke

- Формат invite от `board-token` совпадает с тем, что принимает
  `/board/join/[token]` (одна и та же invite-система — ожидаемо да).
- `StudentGate` не блокирует переход на `/board/*` (страница гостевая, вне
  `/student` — но StudentGate ограничивает только внутренние редиректы, внешний
  `push` проходит; подтвердить).
- Ротация invite при каждом вызове `board-token` не рвёт активную сессию
  репетитора (репетитор входит по JWT, не по invite — см. комментарий в
  `StudentBoardToken`).

---

## Порядок реализации

1. **Задачи** — полный слой (migration → model → repo → service → handler →
   router → frontend tutor+student). Самостоятельная фаза.
2. **Цикл** — `Paid` в модель + `ListLessons` + frontend бейдж/сводка.
3. **Доска** — только frontend-кнопка.

Фичи независимы; порядок между 2 и 3 произвольный. Фича 1 — приоритетная по
объёму.
