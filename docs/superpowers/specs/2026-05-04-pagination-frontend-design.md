# Pagination Frontend Design

**Date:** 2026-05-04
**Status:** Approved

## Overview

Add server-side pagination and search to the Students, Courses, and Payments list pages. Optimize the Dashboard to use `total` from paginated responses instead of fetching all records for counts. Pagination state is stored in the URL.

## Scope

**Pages affected:**
- `/students` — pagination + server-side search (name, email)
- `/courses` — pagination + server-side search (subject)
- `/payments` — pagination (no search); replace per-course `useQueries` pattern with single paginated request
- `/dashboard` — replace full-list fetches for counts with lightweight `page=1&limit=1` requests

**Not in scope:** Calendar, individual student/course detail pages, lessons.

## Architecture

### URL State Pattern

All paginated pages use `useSearchParams` + `useRouter` from Next.js. State lives in the URL:

```
/students?page=2&search=иван
/courses?page=1&search=математика
/payments?page=3
```

- Changing the search input resets `page` to `1`
- Search input is debounced 300 ms before writing to URL
- Browser back/forward navigates between pages correctly
- Page state survives refresh

### TanStack Query Cache

Each unique `{page, limit, search}` combination is a separate cache entry:

```ts
queryKey: ['students', { page, limit, search }]
```

Navigating back to a previously visited page hits the cache instantly.

## Backend Changes (Go)

### Add `search` param to Students

`repository/student.go` — extend `GetAll` signature to accept `search string`:

```sql
WHERE tutor_id = $1
  AND ($3 = '' OR first_name ILIKE '%' || $3 || '%'
               OR last_name  ILIKE '%' || $3 || '%'
               OR email      ILIKE '%' || $3 || '%')
ORDER BY first_name
LIMIT $2 OFFSET ...
```

The `COUNT(*)` query gets the same `AND` clause so `total` reflects filtered results.

### Add `search` param to Courses

`repository/course.go` — same pattern, search on `subject ILIKE '%' || $3 || '%'`.

### Payments (no backend change needed)

`GET /payments?page=1&limit=20` already exists and is tutor-scoped. Frontend just switches to using it.

### No migrations required

Search uses `ILIKE` with `%` wildcards — no new indexes needed at expected data volumes for a single tutor's records.

## Frontend Changes

### New: `types/api.ts`

Add generic type:

```ts
export interface PagedResponse<T> {
  data:  T[]
  total: number
  page:  number
  limit: number
}
```

### New: `components/common/Pagination.tsx`

Reusable numbered pagination control. Props:

```ts
interface PaginationProps {
  page:         number
  totalPages:   number
  onPageChange: (page: number) => void
}
```

Renders: `← 1 2 [3] 4 … 8 →`
- Ellipsis shown when total pages > 7
- Prev/Next buttons disabled at boundaries
- Active page highlighted

### Updated: API functions

All list functions return full `PagedResponse<T>` and accept params:

```ts
// students
list: (p: { page: number; limit: number; search: string }) =>
  api.get<PagedResponse<Student>>('/students', { params: p }).then(r => r.data)

// courses — same shape
// payments — same shape, no search param
```

### Updated: Hooks

`useStudents`, `useCourses`, `usePayments` read URL search params and include them in the query key:

```ts
export function useStudents(params: { page: number; limit: number; search: string }) {
  return useQuery({
    queryKey: studentKeys.list(params),
    queryFn:  () => studentsApi.list(params),
  })
}
```

### Updated: List pages

`students/page.tsx`, `courses/page.tsx`, `payments/page.tsx`:
- Read `page`, `search` from `useSearchParams()`
- Remove `useState` for those values
- Remove client-side `.filter()` for search
- Pass `data`, `total`, `page`, `limit` from hook response to `<Pagination>`
- Search input writes debounced value to URL (`router.replace`)

### Updated: Dashboard

Replace `useStudents()` and `useCourses()` (which fetch all records) with a new lightweight hook:

```ts
function useResourceCount(resource: 'students' | 'courses'): number
```

Internally requests `?page=1&limit=1` and returns `response.total`. This drops the payload from potentially hundreds of records to a single item.

For active courses count (`ended_at === null`), the dashboard currently filters client-side. With this change it shows the total course count (not filtered by active). If filtering by active is required later, a dedicated backend param can be added.

## Data Flow: Search

```
User types in search box
  → 300ms debounce
  → router.replace('?search=<value>&page=1')
  → useSearchParams() updates
  → TanStack Query sees new queryKey
  → API request: GET /students?search=иван&page=1&limit=20
  → Response: { data: [...], total: 3, page: 1, limit: 20 }
  → Table re-renders with filtered rows
  → Pagination shows "страница 1 из 1"
```

## Data Flow: Page Navigation

```
User clicks page 3 in Pagination component
  → onPageChange(3)
  → router.replace('?page=3&search=<current>')
  → useSearchParams() updates
  → TanStack Query checks cache for { page: 3, search: '...' }
  → Cache miss → API request: GET /students?page=3&limit=20
  → Table updates
```

## Default Values

| Param   | Default | Max  |
|---------|---------|------|
| `page`  | 1       | —    |
| `limit` | 20      | 100  |

`limit` is not user-configurable in the UI — fixed at 20 per page.
