# Кабинет ученика v2 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Добавить в кабинет ученика задачи (ДЗ к урокам), бейджи оплаты цикла и кнопку входа на доску.

**Architecture:** Три независимые фичи. Backend задач — полный слой (`lesson_tasks`). Цикл — один флаг `Paid` из уже считающейся позиции цикла. Доска — только frontend поверх готового `board-token` эндпоинта. Spec: `docs/superpowers/specs/2026-07-12-student-cabinet-v2-design.md`.

**Tech Stack:** Go + Gin + pgxpool + goose (backend); Next.js + React Query + TS (frontend). Тесты backend — testify/mock, `package service_test` / handler-тесты в `handlers/`.

## Global Constraints

- Слои: `main.go → router → handlers → services → repositories → pgxpool`. Проводка — только в `router/router.go`, без глобалей.
- Tutor-роуты под `middleware.Auth` (`c.GetString("tutorID")`), student-роуты под `middleware.AuthStudent` (`c.GetString("studentID")`). Всегда nil-guard → 401.
- Валидация тела: `bindAndValidate(c, &req)` из `handlers/helpers.go`.
- Authz fails-closed: ученик касается задачи только через `EnrolledInLesson(studentID, lessonID)`; репетитор — только через владение уроком (JOIN `lessons→courses.tutor_id`).
- Миграции goose: `-- +goose Up` / `-- +goose Down`, оба направления. Следующий номер — `022`.
- Frontend student API — только через `studentHttp` (`lib/api/studentClient.ts`); типы в `types/api.ts`.
- Паттерн добавления фич: migration → model → repo → service → handler → router (см. существующие `student.go` в каждом слое как образец).

## Параллелизация (для диспетчера субагентов)

- **Stream A — Backend задач** (Tasks A1→A4): последовательная цепочка (repo←model, service←repo, handler←service). Один воркстрим.
- **Stream B — Backend цикла** (Task B1): независим, другие файлы (`models/lesson.go`, `service/student.go`). **Параллельно со Stream A.**
- **Stream C — Frontend** (Tasks C1→C4): зависит от A и B (нужны роуты + поле `paid`). Один воркстрим — все задачи трогают `student/(cabinet)/lessons/page.tsx`, параллелить между собой нельзя.

Порядок: (A ‖ B) → C.

---

## STREAM A — Backend задач

### Task A1: Миграция + модель

**Files:**
- Create: `migrations/022_lesson_tasks.sql`
- Modify: `models/task.go` (Create)

**Interfaces:**
- Produces: `models.LessonTask`, `models.CreateTaskRequest`, `models.UpdateTaskRequest`, `models.SetTaskDoneRequest`.

- [ ] **Step 1: Миграция**

`migrations/022_lesson_tasks.sql`:
```sql
-- +goose Up
CREATE TABLE lesson_tasks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lesson_id   UUID NOT NULL REFERENCES lessons(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    done        BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX lesson_tasks_lesson_id_idx ON lesson_tasks(lesson_id);

-- +goose Down
DROP TABLE IF EXISTS lesson_tasks;
```

- [ ] **Step 2: Применить миграцию**

Run: `make migrate-up`
Expected: `OK 022_lesson_tasks.sql`. Проверить `make migrate-status`.

- [ ] **Step 3: Модель**

`models/task.go`:
```go
package models

import "time"

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

type UpdateTaskRequest struct {
	Title       string `json:"title"       validate:"required,min=1,max=200"`
	Description string `json:"description" validate:"omitempty,max=1000"`
}

type SetTaskDoneRequest struct {
	Done bool `json:"done"`
}
```

- [ ] **Step 4: Сборка**

Run: `go build ./...`
Expected: без ошибок.

- [ ] **Step 5: Commit**

```bash
git add migrations/022_lesson_tasks.sql models/task.go
git commit -m "feat(tasks): lesson_tasks migration + models"
```

---

### Task A2: Repository

**Files:**
- Create: `repository/task.go`

**Interfaces:**
- Consumes: `models.LessonTask`, `models.CreateTaskRequest`, `models.UpdateTaskRequest`.
- Produces: `repository.TaskRepository` со следующими методами (все authz-условия — в SQL):

```go
type TaskRepository interface {
	// tutor: список задач урока, если урок принадлежит tutorID
	ListByLessonForTutor(ctx context.Context, lessonID, tutorID string) ([]models.LessonTask, error)
	// tutor: создать задачу, если урок принадлежит tutorID; иначе ErrForbidden-семантика (0 строк)
	Create(ctx context.Context, lessonID, tutorID string, req models.CreateTaskRequest) (models.LessonTask, error)
	// tutor: обновить текст, если задача на уроке tutorID
	Update(ctx context.Context, taskID, tutorID string, req models.UpdateTaskRequest) (models.LessonTask, error)
	// tutor: удалить, если задача на уроке tutorID; вернуть кол-во удалённых
	Delete(ctx context.Context, taskID, tutorID string) (int64, error)
	// student: список задач урока, в котором ученик enrolled
	ListByLessonForStudent(ctx context.Context, lessonID string) ([]models.LessonTask, error)
	// student: сменить done; вернуть кол-во обновлённых (0 если задача не найдена)
	SetDone(ctx context.Context, taskID string, done bool) (int64, error)
	// student: lesson_id задачи (для проверки enrollment перед SetDone)
	LessonIDByTask(ctx context.Context, taskID string) (string, error)
}
```

- [ ] **Step 1: Реализация repo**

`repository/task.go` — зеркалить стиль `repository/student.go` (struct `taskRepository{conn *pgxpool.Pool}`, `NewTaskRepository`). Ключевые запросы:

```go
// Create — проверка владения уроком в самом INSERT через SELECT-подзапрос
const q = `
INSERT INTO lesson_tasks (lesson_id, title, description)
SELECT l.id, $3, $4
  FROM lessons l JOIN courses c ON c.id = l.course_id
 WHERE l.id = $1 AND c.tutor_id = $2
RETURNING id, lesson_id, title, description, done, created_at`
// req → $3 title, $4 description; $1 lessonID, $2 tutorID
// pgx.ErrNoRows ⇒ урок не принадлежит tutorID ⇒ вернуть (LessonTask{}, pgx.ErrNoRows)
```

```go
// ListByLessonForTutor
`SELECT t.id, t.lesson_id, t.title, t.description, t.done, t.created_at
   FROM lesson_tasks t JOIN lessons l ON l.id = t.lesson_id
                       JOIN courses c ON c.id = l.course_id
  WHERE t.lesson_id = $1 AND c.tutor_id = $2
  ORDER BY t.created_at`

// Update
`UPDATE lesson_tasks t SET title=$3, description=$4
   FROM lessons l, courses c
  WHERE t.id=$1 AND l.id=t.lesson_id AND c.id=l.course_id AND c.tutor_id=$2
RETURNING t.id, t.lesson_id, t.title, t.description, t.done, t.created_at`

// Delete → r.conn.Exec(...); return tag.RowsAffected()
`DELETE FROM lesson_tasks t USING lessons l, courses c
  WHERE t.id=$1 AND l.id=t.lesson_id AND c.id=l.course_id AND c.tutor_id=$2`

// ListByLessonForStudent (enrollment уже проверен в service) — просто по lesson_id
`SELECT id, lesson_id, title, description, done, created_at
   FROM lesson_tasks WHERE lesson_id=$1 ORDER BY created_at`

// SetDone → Exec; return tag.RowsAffected()
`UPDATE lesson_tasks SET done=$2 WHERE id=$1`

// LessonIDByTask
`SELECT lesson_id FROM lesson_tasks WHERE id=$1`
```

- [ ] **Step 2: Сборка**

Run: `go build ./...`
Expected: без ошибок.

- [ ] **Step 3: Commit**

```bash
git add repository/task.go
git commit -m "feat(tasks): TaskRepository"
```

---

### Task A3: Service + тесты

**Files:**
- Create: `service/task.go`, `service/task_test.go`

**Interfaces:**
- Consumes: `repository.TaskRepository`, `repository.StudentRepository` (для `EnrolledInLesson`).
- Produces:

```go
type TaskService interface {
	ListForTutor(ctx context.Context, lessonID, tutorID string) ([]models.LessonTask, error)
	Create(ctx context.Context, lessonID, tutorID string, req models.CreateTaskRequest) (models.LessonTask, error)
	Update(ctx context.Context, taskID, tutorID string, req models.UpdateTaskRequest) (models.LessonTask, error)
	Delete(ctx context.Context, taskID, tutorID string) error
	ListForStudent(ctx context.Context, lessonID, studentID string) ([]models.LessonTask, error)
	SetDone(ctx context.Context, taskID, studentID string, done bool) error
}
```

Семантика ошибок (sentinel из `service/errors.go`: `ErrNotFound`, `ErrForbidden` — оба существуют):
- tutor Create/Update при `pgx.ErrNoRows` из repo → `ErrNotFound`.
- tutor Delete при `RowsAffected==0` → `ErrNotFound`.
- student `ListForStudent`/`SetDone`: сначала `EnrolledInLesson`; не enrolled → `ErrForbidden`. Для `SetDone` lessonID берётся через `repo.LessonIDByTask`; задача не найдена → `ErrNotFound`.

- [ ] **Step 1: Failing-тест (service)**

`service/task_test.go` (`package service_test`), моки `TaskRepository` и `StudentRepository` (зеркалить существующие моки в `service/student_test.go`). Кейсы:
```
TestTask_StudentSetDone_NotEnrolled  → EnrolledInLesson=false ⇒ ошибка, repo.SetDone НЕ вызван
TestTask_StudentSetDone_OK           → enrolled=true ⇒ repo.SetDone(taskID,true) вызван, err=nil
TestTask_StudentList_NotEnrolled     → EnrolledInLesson=false ⇒ ошибка, repo.ListByLessonForStudent НЕ вызван
TestTask_TutorCreate_NotOwned        → repo.Create возвращает pgx.ErrNoRows ⇒ service возвращает ErrNotFound
```
Мок `SetDone` для enrolled-кейса возвращает `(int64(1), nil)`. Для `SetDone` замокать `LessonIDByTask`→(lessonID,nil).

- [ ] **Step 2: Прогнать — падает**

Run: `go test ./service/ -run TestTask -v`
Expected: FAIL (нет `NewTaskService`).

- [ ] **Step 3: Реализация service**

`service/task.go` — `taskService{repo repository.TaskRepository; studentRepo repository.StudentRepository}`, `NewTaskService(repo, studentRepo)`. `SetDone`:
```go
func (s *taskService) SetDone(ctx context.Context, taskID, studentID string, done bool) error {
	lessonID, err := s.repo.LessonIDByTask(ctx, taskID)
	if err != nil { return fmt.Errorf("task: %w", ErrNotFound) }
	ok, err := s.studentRepo.EnrolledInLesson(ctx, studentID, lessonID)
	if err != nil { return err }
	if !ok { return fmt.Errorf("task: %w", ErrForbidden) }
	n, err := s.repo.SetDone(ctx, taskID, done)
	if err != nil { return err }
	if n == 0 { return fmt.Errorf("task: %w", ErrNotFound) }
	return nil
}
```
`ListForStudent` — сперва `EnrolledInLesson(studentID, lessonID)`, потом `repo.ListByLessonForStudent`. Tutor-методы — прямой проброс + маппинг `pgx.ErrNoRows`/`RowsAffected==0` в `ErrNotFound`.

- [ ] **Step 4: Прогнать — проходит**

Run: `go test ./service/ -run TestTask -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add service/task.go service/task_test.go
git commit -m "feat(tasks): TaskService + tests"
```

---

### Task A4: Handlers + router

**Files:**
- Create: `handlers/task.go`, `handlers/task_test.go`
- Modify: `router/router.go`

**Interfaces:**
- Consumes: `service.TaskService`, `service.ErrNotFound`/`ErrForbidden`.
- Роуты:
  - tutor (group `auth`): `GET /lessons/:id/tasks`, `POST /lessons/:id/tasks`, `PUT /tasks/:id`, `DELETE /tasks/:id`
  - student (group `stu`): `GET /student/lessons/:id/tasks`, `PATCH /student/tasks/:id`

- [ ] **Step 1: Handler**

`handlers/task.go` — `TaskHandler{svc service.TaskService; log ...}`, `NewTaskHandler`. Методы читают id из `c.Param("id")`, tutorID/studentID из контекста с nil-guard→401, тело через `bindAndValidate`. Маппинг ошибок: `ErrNotFound`→404, `ErrForbidden`→403, прочее→500. Пример student PATCH:
```go
func (h *TaskHandler) StudentSetDone(c *gin.Context) {
	studentID := c.GetString("studentID")
	if studentID == "" { c.JSON(401, gin.H{"error": "unauthorized"}); return }
	var req models.SetTaskDoneRequest
	if !bindAndValidate(c, &req) { return }
	err := h.svc.SetDone(c.Request.Context(), c.Param("id"), studentID, req.Done)
	switch {
	case errors.Is(err, service.ErrNotFound): c.JSON(404, gin.H{"error": "not found"})
	case errors.Is(err, service.ErrForbidden): c.JSON(403, gin.H{"error": "forbidden"})
	case err != nil: c.JSON(500, gin.H{"error": "internal"})
	default: c.Status(204)
	}
}
```

- [ ] **Step 2: Router wiring**

`router/router.go`:
```go
// рядом с прочей проводкой:
taskRepo := repository.NewTaskRepository(pool)
taskService := service.NewTaskService(taskRepo, studentRepo)
taskHandler := handlers.NewTaskHandler(taskService, log)
// в группе auth (tutor):
auth.GET("/lessons/:id/tasks", taskHandler.ListForTutor)
auth.POST("/lessons/:id/tasks", taskHandler.Create)
auth.PUT("/tasks/:id", taskHandler.Update)
auth.DELETE("/tasks/:id", taskHandler.Delete)
// в группе stu (student):
stu.GET("/lessons/:id/tasks", taskHandler.ListForStudent)
stu.PATCH("/tasks/:id", taskHandler.StudentSetDone)
```
ВНИМАНИЕ: проверить, что `GET /lessons/:id/tasks` не конфликтует с существующим `GET /lessons/:id` (Gin разрешает — разные сегменты). При панике роутера — согласовать имена wildcard-параметров с соседними роутами (`:id` уже используется на `/lessons/:id`).

- [ ] **Step 3: Handler-тест**

`handlers/task_test.go` — зеркалить `handlers/student_auth_test.go` (мок `TaskService`, `makeRequest`). Кейсы: `StudentSetDone` без studentID→401; сервис `ErrForbidden`→403; успех→204. Tutor `Create` с невалидным телом (пустой title)→400.

- [ ] **Step 4: Прогон + сборка**

Run: `go build ./... && go test ./handlers/ ./service/ -run Task -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add handlers/task.go handlers/task_test.go router/router.go
git commit -m "feat(tasks): handlers + routes"
```

---

## STREAM B — Backend цикла (параллельно со Stream A)

### Task B1: Флаг `Paid` в CalendarLesson

**Files:**
- Modify: `models/lesson.go` (CalendarLesson), `service/student.go` (ListLessons)
- Modify: `service/student_test.go`

**Interfaces:**
- Produces: `CalendarLesson.Paid *bool` (JSON `paid`, omitempty).

- [ ] **Step 1: Failing-тест**

В `service/student_test.go` добавить `TestListLessons_PaidFlag`: мок `StudentRepository.ListLessons` возвращает уроки с `Rank` 1,2,3; мок `PaymentRepository.GetByCoursesBatch` — платёж(и) с суммарным `LessonsCount=2`. Ожидания: урок rank1 `Paid==true`, rank2 `Paid==true`, rank3 `Paid==false`. (Зеркалить существующие моки в этом файле.)

- [ ] **Step 2: Прогнать — падает**

Run: `go test ./service/ -run TestListLessons_PaidFlag -v`
Expected: FAIL (нет поля `Paid`).

- [ ] **Step 3: Модель**

`models/lesson.go`, в `CalendarLesson` после `CycleSize`:
```go
	Paid *bool `json:"paid,omitempty"`
```

- [ ] **Step 4: Логика в ListLessons**

`service/student.go`, в цикле обогащения (`for i, l := range lessons` где `l.Rank != nil`), там же где считается `pos`:
```go
pos, size := cyclePositionFromRank(*l.Rank, coursePayments)
paid := pos > 0
lessons[i].Paid = &paid
if pos > 0 {
	p, sz := pos, size
	lessons[i].CyclePosition = &p
	lessons[i].CycleSize = &sz
}
```
Урок без `Rank` или без платежей курса — `Paid` остаётся `nil` (бейдж не показываем).

- [ ] **Step 5: Прогнать — проходит**

Run: `go test ./service/ -run TestListLessons -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add models/lesson.go service/student.go service/student_test.go
git commit -m "feat(cycle): expose Paid flag on student lessons"
```

---

## STREAM C — Frontend (после A и B)

### Task C1: Типы + student API (задачи, доска)

**Files:**
- Modify: `frontend/src/types/api.ts` (CalendarLesson + LessonTask), `frontend/src/lib/api/student.ts`

**Interfaces:**
- Produces: `studentApi.tasks(lessonId)`, `studentApi.setTaskDone(taskId, done)`, `studentApi.boardToken(lessonId)`; типы `LessonTask`, `CalendarLesson.paid?`.

- [ ] **Step 1: Типы**

В `types/api.ts`: в `CalendarLesson` добавить `paid?: boolean`. Новый тип:
```ts
export interface LessonTask {
  id: string
  lesson_id: string
  title: string
  description: string
  done: boolean
  created_at: string
}
```

- [ ] **Step 2: API-методы**

В `lib/api/student.ts` в объект `studentApi`:
```ts
  tasks: (lessonId: string) =>
    studentHttp.get<LessonTask[]>(`/student/lessons/${lessonId}/tasks`).then((r) => r.data),
  setTaskDone: (taskId: string, done: boolean) =>
    studentHttp.patch<void>(`/student/tasks/${taskId}`, { done }),
  boardToken: (lessonId: string) =>
    studentHttp
      .get<{ invite_token: string; page_id: string }>(`/student/lessons/${lessonId}/board-token`)
      .then((r) => r.data),
```
Импорт `LessonTask` из `@/types/api`.

- [ ] **Step 3: Сборка**

Run: `cd frontend && npx tsc --noEmit`
Expected: без ошибок.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/types/api.ts frontend/src/lib/api/student.ts
git commit -m "feat(student-fe): task + board-token API types"
```

---

### Task C2: Бейдж оплаты + сводка цикла

**Files:**
- Modify: `frontend/src/app/student/(cabinet)/lessons/page.tsx`

- [ ] **Step 1: Бейдж на карточке**

В `LessonRow`, в блок бейджей после cycle-position badge:
```tsx
{lesson.paid === true && <Badge variant="secondary">Оплачен</Badge>}
{lesson.paid === false && <Badge variant="outline">Не оплачен</Badge>}
```

- [ ] **Step 2: Сводка текущего цикла**

В `LessonsInner`, только для `tab==='upcoming'`, вычислить из первого предстоящего урока с `cycle_position`/`cycle_size`:
```tsx
const cur = lessons?.find((l) => l.cycle_position != null && l.cycle_size != null)
const cycleSummary =
  cur && cur.cycle_position != null && cur.cycle_size != null
    ? `Оплачено ${cur.cycle_position} из ${cur.cycle_size}, осталось ${cur.cycle_size - cur.cycle_position}`
    : null
```
Показать строкой над `SectionCard` (стиль как у muted-текста), если `cycleSummary && tab==='upcoming'`.

- [ ] **Step 3: Проверка в браузере**

Run: `cd frontend && npm run dev` → войти учеником, открыть `/student/lessons`.
Expected: у оплаченных уроков «Оплачен», у выходящих за оплату «Не оплачен», сверху строка «Оплачено N из M, осталось K».

- [ ] **Step 4: Commit**

```bash
git add frontend/src/app/student/\(cabinet\)/lessons/page.tsx
git commit -m "feat(student-fe): paid badges + cycle summary"
```

---

### Task C3: Кнопка доски + задачи на карточке ученика

**Files:**
- Modify: `frontend/src/app/student/(cabinet)/lessons/page.tsx`

- [ ] **Step 1: Кнопка «Доска»**

В `LessonRow` рядом с «Войти в урок» (для `upcoming && status==='scheduled'`):
```tsx
<Button size="sm" variant="outline" onClick={async () => {
  const { invite_token } = await studentApi.boardToken(lesson.id)
  router.push(`/board/join/${invite_token}`)
}}>
  Доска
</Button>
```
Обернуть в try/catch с toast об ошибке (как в существующих обработчиках). Импортировать `studentApi`.

- [ ] **Step 2: Задачи на карточке**

Под строкой с датой в `LessonRow` — подгрузить задачи `useQuery(['student-tasks', lesson.id], () => studentApi.tasks(lesson.id))` и отрисовать список с чекбоксом:
```tsx
<label className="flex items-center gap-2" style={{ fontSize: 13 }}>
  <input type="checkbox" checked={t.done} onChange={(e) => setDone.mutate({ id: t.id, done: e.target.checked })} />
  <span style={{ textDecoration: t.done ? 'line-through' : 'none' }}>{t.title}</span>
</label>
```
`setDone` — `useMutation(({id,done}) => studentApi.setTaskDone(id, done))` с `onMutate` оптимистично меняющим кэш `['student-tasks', lesson.id]` и `onError` откатом (стандартный React Query optimistic-паттерн). Ленивое упрощение: если задач нет — блок не рисуем.

- [ ] **Step 3: Проверка в браузере**

Открыть `/student/lessons`: кнопка «Доска» открывает доску (`/board/join/...`); задачи видны, чекбокс переключает и сохраняется после refetch.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/app/student/\(cabinet\)/lessons/page.tsx
git commit -m "feat(student-fe): board button + task checklist on lesson card"
```

---

### Task C4: Задачи в UI репетитора

**Files:**
- Modify: `frontend/src/components/lessons/LessonForm.tsx` (форма редактирования урока репетитора)
- Modify: `frontend/src/lib/api/lessons.ts` (tutor API, клиент `api` из `./client`)

- [ ] **Step 1: Tutor API**

В `lib/api/lessons.ts` (импорт `api` из `./client`, `LessonTask` из `@/types/api`):
```ts
tasks: (lessonId: string) => api.get<LessonTask[]>(`/lessons/${lessonId}/tasks`).then(r => r.data),
createTask: (lessonId: string, data: {title: string; description?: string}) =>
  api.post<LessonTask>(`/lessons/${lessonId}/tasks`, data).then(r => r.data),
updateTask: (taskId: string, data: {title: string; description?: string}) =>
  api.put<LessonTask>(`/tasks/${taskId}`, data).then(r => r.data),
deleteTask: (taskId: string) => api.delete(`/tasks/${taskId}`),
```

- [ ] **Step 2: UI-блок в LessonForm**

В `components/lessons/LessonForm.tsx` — блок «Задачи» (только для существующего урока, т.е. когда есть `lesson.id`): список задач (title + кнопка удалить) и инпут «добавить задачу». Следовать стилю формы. React Query для списка/мутаций с инвалидацией `['lesson-tasks', lessonId]`.

- [ ] **Step 3: Проверка в браузере**

Репетитор: открыть урок → добавить задачу → она появляется; удалить → исчезает. Затем войти учеником — задача видна на карточке урока.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/api/lessons.ts frontend/src/components/lessons/LessonForm.tsx
git commit -m "feat(tutor-fe): manage lesson tasks in lesson editor"
```

---

## Финальный smoke (после всех стримов)

- [ ] Репетитор ставит задачу к уроку → ученик видит → отмечает done → репетитор видит статус.
- [ ] Бейджи «Оплачен/Не оплачен» и сводка цикла корректны относительно платежей курса.
- [ ] Кнопка «Доска» у ученика открывает `/board/join/...`, доска грузится, `StudentGate` не блокирует.
- [ ] Ротация board-invite не рвёт активную сессию репетитора на доске.
