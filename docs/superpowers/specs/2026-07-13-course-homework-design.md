# Домашнее задание (markdown, на уровне курса)

**Дата:** 2026-07-13
**Статус:** утверждён

Заменяет per-lesson фичу `LessonTask` (спека `2026-07-12-student-cabinet-v2`,
раздел «Задачи»): дизайн «задача↔урок, ученик отмечает done» не подошёл. Новый
подход — одно markdown-ДЗ на курс, которое препод редактирует и ученик читает.

Фичи «бейджи оплаты цикла» и «вход на доску» из v2 остаются — трогаем только
задачи.

## Решения

1. **Одно ДЗ на курс**, перезапись при сохранении. Без истории, без отдельной
   таблицы — колонка на `courses`.
2. **Удаляем** смерженную `LessonTask`-фичу целиком (backend + frontend).
3. **Новая зависимость** — `react-markdown` (+ `rehype-sanitize`) для рендера ДЗ
   у ученика. Препод пишет в обычном `textarea`.

## Часть 0 — удаление LessonTask

- Миграция `023`: `DROP TABLE lesson_tasks` (был создан в `022`, уже в проде) +
  `ALTER TABLE courses ADD COLUMN homework`.
- Удалить файлы: `models/lesson_task.go`, `repository/lesson_task.go`,
  `service/lesson_task.go`, `service/lesson_task_test.go`,
  `handlers/lesson_task.go`, `handlers/lesson_task_test.go`.
- `router/router.go`: снять проводку `lessonTask*` и 6 роутов
  (`GET/POST /lessons/:id/tasks`, `PUT/DELETE /lesson-tasks/:id`,
  `GET /student/lessons/:id/tasks`, `PATCH /student/lesson-tasks/:id`).
- Frontend, хирургически (сохранить бейджи/сводку/кнопку доски):
  - `types/api.ts`: убрать `LessonTask`.
  - `lib/api/student.ts`: убрать `tasks`, `setTaskDone` (оставить `boardToken`).
  - `lib/api/lessons.ts`: убрать `tasks/createTask/updateTask/deleteTask`.
  - `app/student/(cabinet)/lessons/page.tsx`: убрать чек-лист задач (оставить
    бейджи `paid`, сводку цикла, кнопку «Доска»).
  - `components/lessons/LessonForm.tsx`: убрать блок «Задачи».

## Часть 1 — данные и модель

Миграция `023` (та же):
```sql
-- +goose Up
DROP TABLE IF EXISTS lesson_tasks;
ALTER TABLE courses ADD COLUMN homework TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE courses DROP COLUMN homework;
-- (lesson_tasks не восстанавливаем — фича удалена)
```

`homework` добавляется в `models.Course` (JSON `homework`), возвращается со всеми
чтениями курса.

## Часть 2 — backend

**Препод** (`middleware.Auth`, владелец курса — как в существующих course-роутах):
- `PUT /courses/:id/homework` — тело `{homework string}` (`validate:"max=20000"`),
  проверка `course.tutor_id == tutorID`, `UPDATE courses SET homework=$1`.

**Ученик** (`middleware.AuthStudent`):
- `GET /student/homework` → `[]StudentHomework{course_id, subject, homework}` по
  курсам, где ученик enrolled (оба пути: `courses.student_id` И
  `course_enrollments`, как в `EnrolledInLesson`) и `homework <> ''`.

Модель:
```go
type StudentHomework struct {
    CourseID string `json:"course_id"`
    Subject  string `json:"subject"`
    Homework string `json:"homework"`
}
type UpdateHomeworkRequest struct {
    Homework string `json:"homework" validate:"max=20000"`
}
```

Тесты (service, testify/mock): препод сохраняет ДЗ чужого курса → `ErrNotFound`;
`GET /student/homework` возвращает только enrolled-курсы с непустым ДЗ.

## Часть 3 — frontend препода

- `lib/api/courses.ts`: `updateHomework(courseId, homework)` → `PUT
  /courses/:id/homework`; `homework` появляется в типе `Course`.
- Кнопка **«Домашнее задание»** на `/courses/[id]` → модалка (`Dialog` +
  `textarea` + «Сохранить», инвалидация курса).
- `components/call/CallToolbar.tsx`: кнопка ДЗ при `role==='tutor'` → та же
  модалка (вынести в общий компонент `HomeworkDialog`, чтобы переиспользовать на
  странице курса и в звонке).

## Часть 4 — frontend ученика

- `lib/api/student.ts`: `homework()` → `GET /student/homework`.
- `app/student/(cabinet)/lessons/page.tsx`: блок **«Домашнее задание»** над
  списком уроков — рендер markdown (`react-markdown` + `rehype-sanitize`) по
  курсам с непустым ДЗ.
- `CallToolbar` при `role==='guest'` → кнопка ДЗ → read-only просмотр (тот же
  markdown-рендер). В звонке ДЗ грузится по `course_id` урока; проще —
  переиспользовать `GET /student/homework` и отфильтровать по курсу текущего
  урока (course_id уже есть в данных урока звонка).

## Порядок

0 (удаление) → 1 (миграция+модель) → 2 (backend) → 3 (frontend препод) →
4 (frontend ученик). Части 3 и 4 частично независимы (разные файлы), кроме общего
`HomeworkDialog` и `student.ts`.
