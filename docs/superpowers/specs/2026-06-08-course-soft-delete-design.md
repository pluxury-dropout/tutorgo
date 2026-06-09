# Course Soft Delete Design

**Date:** 2026-06-08
**Status:** Approved

## Problem

Deleting a course currently issues `DELETE FROM courses`, which cascades to all lessons via `ON DELETE CASCADE`. Completed lessons are lost, making historical tracking impossible.

## Goal

Replace hard-delete with soft-delete: archived courses get `is_active = FALSE`, their lessons are untouched, and completed lessons remain visible in the calendar. Archived courses can be restored.

## Database

**Migration `013_course_soft_delete.sql`:**
```sql
ALTER TABLE courses ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT TRUE;
```

No changes to the `lessons` table.

## Go Backend

### Model (`models/course.go`)
Add `IsActive bool` to `Course` struct.

### Repository (`repository/course.go`)
| Method | Change |
|--------|--------|
| `GetAll` | Add `WHERE is_active = TRUE` |
| `Delete` | Change to `UPDATE courses SET is_active = FALSE WHERE id=$1 AND tutor_id=$2` |
| `GetAllArchived` (new) | `WHERE is_active = FALSE AND tutor_id=$1`, same pagination as `GetAll` |
| `Restore` (new) | `UPDATE courses SET is_active = TRUE WHERE id=$1 AND tutor_id=$2` |

### Service (`service/course.go`)
- `Delete`: remove the scheduled-lesson guard entirely — archiving is always safe.
- `GetArchived` (new): calls `repo.GetAllArchived`.
- `Restore` (new): calls `repo.Restore`; returns `ErrNotFound` if course not found.
- `CreateLesson` check (in lesson service): verify `course.IsActive == true` before creating a lesson; return `ErrConflict` if inactive.

### API (`handlers/` + `router/router.go`)
| Endpoint | Handler | Description |
|----------|---------|-------------|
| `DELETE /courses/:id` | existing | now soft-deletes |
| `GET /courses/archived` | `CourseHandler.GetArchived` (new) | paginated archived list |
| `POST /courses/:id/restore` | `CourseHandler.Restore` (new) | reactivates course |

`GET /courses/archived` accepts the same `page`, `limit`, `search` query params as `GET /courses`.

## Frontend

### Types (`types/api.ts`)
Add `is_active: boolean` to `Course` interface.

### Hooks (`lib/hooks/useCourses.ts`)
- `useArchivedCoursesPaged(page, limit, search)` — new hook, mirrors `useCoursesPaged`.
- `useRestoreCourse()` — mutation calling `POST /courses/:id/restore`, invalidates both `courses` and `courses/archived` cache keys.

### Courses page (`app/(dashboard)/courses/page.tsx`)
Add a two-tab switcher **Активные / Архив** at the top of the page.
- **Активные tab**: existing course list with edit/delete icons.
- **Архив tab**: same card layout, but edit/delete icons replaced by a single «Восстановить» button that calls `useRestoreCourse`.

### Calendar
No changes required. Calendar queries lessons by `tutor_id` and date range — lessons from archived courses are already visible.

## Constraints

- Creating a lesson on an archived course returns `409 Conflict` with message `"course is archived"`.
- Restoring a course does not change lesson statuses.
- Archived courses do not appear in dashboard widgets (current cycles, etc.) — they already filter by active course logic via joins with `is_active = TRUE`.

## Out of Scope

- Bulk archive/restore.
- Permanent delete option.
- Showing archive badge on calendar lesson cards.
