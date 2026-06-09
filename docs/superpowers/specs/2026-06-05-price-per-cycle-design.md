# Spec: Price Per Cycle

**Date:** 2026-06-05
**Status:** Approved

## Problem

Tutors often receive a fixed monthly sum (e.g. 14 500 RUB → converted to KZT yields an uneven number), then need to divide by lesson count to get a per-lesson price. This is error-prone and unintuitive. The natural unit of payment in the app is a *cycle* — a fixed repeating block of lessons paid together.

## Goal

Replace `price_per_lesson` with two fields on a course:
- `price_per_cycle` — total amount paid for one cycle
- `lessons_per_cycle` — how many lessons are in one cycle (set by the tutor)

The internal per-lesson price (`price_per_cycle / lessons_per_cycle`) is used only for payment balance calculations and is never exposed to the user directly.

A future widget will compare courses by **price per 60 minutes**, computed from stored cycle fields plus the actual average `duration_minutes` of lessons.

## Data Model

### Migration (`012_price_per_cycle.sql`)

```sql
ALTER TABLE courses
  ADD COLUMN price_per_cycle   numeric(10,2),
  ADD COLUMN lessons_per_cycle int;

-- Migrate existing data: 1 lesson = 1 cycle
UPDATE courses SET price_per_cycle = price_per_lesson, lessons_per_cycle = 1;

ALTER TABLE courses
  ALTER COLUMN price_per_cycle   SET NOT NULL,
  ALTER COLUMN lessons_per_cycle SET NOT NULL;

ALTER TABLE courses
  DROP COLUMN price_per_lesson;
```

### Course model (`models/course.go`)

Replace `PricePerLesson float64` with:
```go
PricePerCycle   float64 `json:"price_per_cycle"`
LessonsPerCycle int     `json:"lessons_per_cycle"`
```

Same replacement in `CreateCourseRequest` and `UpdateCourseRequest`:
```go
PricePerCycle   float64 `json:"price_per_cycle"   validate:"required,gt=0"`
LessonsPerCycle int     `json:"lessons_per_cycle" validate:"required,min=1"`
```

## Backend

### repository/course.go

All INSERT / SELECT / UPDATE queries: replace `price_per_lesson` with `price_per_cycle, lessons_per_cycle`. Scan order updated accordingly.

### repository/payment.go — `GetMonthlyExpected`

```sql
-- Before
SELECT COALESCE(SUM(c.price_per_lesson * lc.cnt), 0) ...

-- After
SELECT COALESCE(SUM((c.price_per_cycle::float / c.lessons_per_cycle) * lc.cnt), 0) ...
```

No other payment queries are affected — `LessonsCount` on payments and balance calculations remain lesson-based.

### Handlers / router

No structural changes. Request/response JSON field names change from `price_per_lesson` to `price_per_cycle` + `lessons_per_cycle`.

## Frontend

### types/api.ts

```ts
// Before
price_per_lesson: number

// After
price_per_cycle:   number
lessons_per_cycle: number
```

### schemas/course.ts

```ts
price_per_cycle:   z.number().positive('Должно быть больше 0'),
lessons_per_cycle: z.number().int().min(1, 'Минимум 1 урок'),
```

### CourseForm.tsx

Replace the single "Цена за урок" input with two inputs:
- **Цена за цикл (₸)** — `price_per_cycle`
- **Уроков в цикле** — `lessons_per_cycle`

Below them, a live read-only hint:
```
= {(price_per_cycle / lessons_per_cycle).toFixed(0)} ₸ за урок
```
Hint is hidden if either field is zero/invalid.

## Future: Price Comparison Widget

Endpoint: `GET /courses/price-comparison`

Response per course:
```json
{
  "course_id":        "...",
  "subject":          "Математика",
  "price_per_cycle":  14500,
  "lessons_per_cycle": 8,
  "avg_duration_min": 60
}
```

`avg_duration_min` = `AVG(duration_minutes)` over non-cancelled lessons of the course.

Frontend computes: `price_per_60min = (price_per_cycle / lessons_per_cycle) / avg_duration_min * 60`

This endpoint and widget are **out of scope** for the current implementation.

## Out of Scope

- Price comparison widget (future feature)
- Changing how payments are recorded (still per `lessons_count`)
- Storing cycle size dynamically from the schedule
