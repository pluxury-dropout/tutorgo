# Средняя ставка за час — дизайн

**Дата:** 2026-06-09  
**Статус:** готов к реализации

## Цель

Показать в виджете «Средний чек» на странице платежей среднюю стоимость одного часа занятий по всем активным курсам репетитора.

## Формула

```
ставка_за_час(курс) = (price_per_cycle / lessons_per_cycle) × (60 / duration_minutes)
итог = AVG(ставка_за_час) по активным курсам с duration_minutes > 0
```

## Архитектура

Расчёт выполняется на фронтенде в `payments/page.tsx` — данные о курсах уже загружены через `useCourses()`. Новый API-эндпоинт не нужен.

Единственное новое поле: `duration_minutes` на модели `Course` (стандартная длительность урока в минутах).

## Стек изменений

```
migrations/014_course_duration.sql
models/course.go
repository/course.go
types/api.ts
schemas/course.ts
lib/api/courses.ts
components/courses/CourseForm.tsx
app/(dashboard)/payments/page.tsx
```

---

## 1. Миграция

**Файл:** `migrations/014_course_duration.sql`

```sql
-- +goose Up
ALTER TABLE courses ADD COLUMN duration_minutes INTEGER NOT NULL DEFAULT 60;

-- +goose Down
ALTER TABLE courses DROP COLUMN duration_minutes;
```

`DEFAULT 60` — все существующие курсы получают 60 минут без ручного заполнения.

---

## 2. Go модель

**Файл:** `models/course.go`

Добавить поле `DurationMinutes int` с тегом `json:"duration_minutes"` в три структуры:

```go
// Course
DurationMinutes int `json:"duration_minutes"`

// CreateCourseRequest
DurationMinutes int `json:"duration_minutes" validate:"required,min=1"`

// UpdateCourseRequest
DurationMinutes int `json:"duration_minutes" validate:"required,min=1"`
```

---

## 3. Репозиторий курсов

**Файл:** `repository/course.go`

Обновить SQL в 6 методах — везде одинаковый паттерн:
1. добавить `duration_minutes` в список колонок SELECT/INSERT/SET
2. добавить `&course.DurationMinutes` в `.Scan()`

| Метод | Изменение |
|---|---|
| `Create` | INSERT + RETURNING: добавить `duration_minutes`, значение `req.DurationMinutes` |
| `GetAll` | SELECT + Scan |
| `GetByID` | SELECT + Scan |
| `GetByStudent` | оба UNION-SELECT + Scan |
| `Update` | SET + RETURNING + Scan |
| `GetAllArchived` | SELECT + Scan |

**Пример для `GetByID` (до/после):**

До:
```go
`SELECT id, student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at, is_active
 FROM courses WHERE id = $1 AND tutor_id = $2`
// Scan: &course.ID, ..., &course.IsActive
```

После:
```go
`SELECT id, student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, duration_minutes, started_at, ended_at, is_active
 FROM courses WHERE id = $1 AND tutor_id = $2`
// Scan: &course.ID, ..., &course.DurationMinutes, ..., &course.IsActive
```

---

## 4. Frontend тип

**Файл:** `frontend/src/types/api.ts`

Добавить в интерфейс `Course`:
```ts
duration_minutes: number
```

---

## 5. Схема валидации

**Файл:** `frontend/src/schemas/course.ts`

Добавить поле в `courseSchema`:
```ts
duration_minutes: z
  .number({ error: 'Введите число' })
  .int('Только целое число')
  .min(1, 'Минимум 1 минута'),
```

---

## 6. API-клиент

**Файл:** `frontend/src/lib/api/courses.ts`

Добавить `duration_minutes: number` в типы CreateInput и UpdateInput (те структуры, что передаются в POST/PUT).

---

## 7. Форма курса

**Файл:** `frontend/src/components/courses/CourseForm.tsx`

- Добавить `duration_minutes: 60` в `defaultValues` и в `reset()`
- Добавить числовое поле рядом с `lessons_per_cycle`:

```tsx
<Label htmlFor="duration_minutes">Длительность урока (мин)</Label>
<Input
  id="duration_minutes"
  type="number"
  min={1}
  {...register('duration_minutes', { valueAsNumber: true })}
/>
{errors.duration_minutes && (
  <p className="text-xs text-destructive">{errors.duration_minutes.message}</p>
)}
```

- При редактировании курса (`initialValues`) добавить `duration_minutes: initial.duration_minutes`.

---

## 8. Страница платежей

**Файл:** `frontend/src/app/(dashboard)/payments/page.tsx`

Добавить расчёт после получения `courses`:

```ts
const avgRate = (() => {
  const active = courses.filter(c => c.is_active && c.duration_minutes > 0)
  if (!active.length) return null
  const sum = active.reduce(
    (acc, c) => acc + (c.price_per_cycle / c.lessons_per_cycle) * (60 / c.duration_minutes),
    0
  )
  return Math.round(sum / active.length)
})()
```

Обновить сегмент `avg` в массиве `segments`:

```ts
{
  id:       'avg',
  label:    'Ср. ставка/час',
  value:    avgRate !== null ? `₸ ${avgRate.toLocaleString('ru-RU')}` : '—',
  dotColor: 'var(--warning)',
  meta:     avgRate !== null ? 'за 60 минут' : 'нет курсов',
},
```

---

## Порядок реализации

1. Создать и применить миграцию `014_course_duration.sql`
2. Обновить `models/course.go`
3. Обновить `repository/course.go`
4. Обновить `types/api.ts`
5. Обновить `schemas/course.ts`
6. Обновить `lib/api/courses.ts`
7. Обновить `CourseForm.tsx`
8. Обновить `payments/page.tsx`

## Проверка

- Создать новый курс с `duration_minutes = 45` → убедиться, что поле сохранилось и вернулось из API
- Открыть страницу платежей → убедиться, что виджет показывает число (не `—`)
- Архивировать курс → убедиться, что он выпал из расчёта
