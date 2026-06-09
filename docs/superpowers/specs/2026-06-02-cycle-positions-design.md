# Cycle Position Badges on Lessons

**Date:** 2026-06-02  
**Status:** Approved

## Problem

A tutor needs to see at a glance which lesson number within the current payment cycle each lesson represents, without navigating into each course. The cycle is defined by payment records: each `Payment` has a `lessons_count` that determines how many lessons belong to that cycle.

## Scope

- Show a cycle position badge on every lesson in the calendar and in the lesson list on the course page.
- The badge shows the ordinal number within the current cycle (e.g. "5").
- The last lesson of a cycle gets a distinct red badge as a visual alert.
- Cancelled lessons do not occupy cycle slots.
- Lessons beyond the total paid count receive no badge.

## Architecture

### Approach

Service-layer computation in Go (Approach B). SQL stays simple; cycle logic lives in testable Go functions.

### New Model Fields

`models/lesson.go` — added to both `Lesson` and `CalendarLesson`:

```go
CyclePosition *int `json:"cycle_position,omitempty"`
CycleSize     *int `json:"cycle_size,omitempty"`
```

`nil` means the lesson has no associated payment cycle (beyond paid count or no payments exist).

### New Repository Methods

**`repository/lesson.go`**

```go
GetRanksForCourses(ctx context.Context, courseIDs []string) (map[string]int, error)
```

Returns `map[lessonID → globalRank]` among non-cancelled lessons of each course, ordered by `scheduled_at ASC`. Uses a single SQL query with `ROW_NUMBER() OVER (PARTITION BY course_id ORDER BY scheduled_at)` and `WHERE status != 'cancelled'`.

**`repository/payment.go`**

```go
GetByCoursesBatch(ctx context.Context, courseIDs []string) (map[string][]models.Payment, error)
```

Returns `map[courseID → []Payment]` ordered by `paid_at ASC` for all given courses in one query.

### Service Layer

**`service/lesson.go`** — new helper:

```go
func computeCyclePositions(
    lessons  []models.Lesson,
    ranks    map[string]int,        // lessonID → 1-based rank (non-cancelled only)
    payments []models.Payment,      // sorted by paid_at ASC
)
```

Algorithm:
1. Build cumulative payment boundaries: `[8, 18, 26, ...]` from `lessons_count` values.
2. For each lesson, look up its rank from `ranks`.
3. Find which payment bucket the rank falls into using binary search over boundaries.
4. Set `CyclePosition = rank - prevBoundary`, `CycleSize = payment.LessonsCount`.
5. If rank exceeds total paid count → leave fields `nil`.

**`GetCalendar`** flow after enrichment:
1. Call existing `lessonRepo.GetCalendar(tutorID, from, to)`.
2. Collect unique `courseIDs`.
3. Call `lessonRepo.GetRanksForCourses(courseIDs)` — one batch query.
4. Call `paymentRepo.GetByCoursesBatch(courseIDs)` — one batch query.
5. Call `computeCyclePositions` per course group.
6. Return enriched `[]CalendarLesson`.

**`GetByCourse` / `GetByCoursePaged`** — same flow, `courseIDs` is a single element.

### Frontend

**New component:** `components/lessons/CycleBadge.tsx`

```tsx
interface CycleBadgeProps {
  position: number
  size: number
}
```

- `position < size` → small neutral grey label in the bottom-right corner.
- `position === size` → small red badge (`rounded-full bg-red-500 text-white`).
- Shape and colour can be adjusted later without changing logic.

**Calendar** (`calendar/page.tsx` → `eventContent`):  
Render `<CycleBadge>` inside the lesson event block when `cycle_position` is present.

**Course page** (`courses/[id]/page.tsx`):  
Render `<CycleBadge>` in each lesson row when `cycle_position` is present.

## Data Flow

```
DB: lessons + payments
        ↓
lessonRepo.GetCalendar()         → []CalendarLesson (no cycle fields yet)
lessonRepo.GetRanksForCourses()  → map[lessonID]rank
paymentRepo.GetByCoursesBatch()  → map[courseID][]Payment
        ↓
service.computeCyclePositions()  → mutates CyclePosition, CycleSize
        ↓
handler returns enriched JSON
        ↓
frontend renders CycleBadge
```

## Error Handling

- If a course has no payments → all lessons get `nil` fields (no badge shown).
- If `GetRanksForCourses` or `GetByCoursesBatch` fails → `GetCalendar` returns an error (no partial results).

## Testing

- Unit test `computeCyclePositions`: multiple payment cycles, cancelled lessons skipped, lessons beyond paid count get nil.
- Existing service tests remain unchanged.
