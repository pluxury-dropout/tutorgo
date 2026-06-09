# Delete Series by Date Range — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an optional upper-bound date (`toDate`) to series deletion so tutors can delete a date range instead of only "from date to end".

**Architecture:** Extend the existing `DeleteSeries` call with one additional optional parameter `toDate *string` threaded through repository → service → handler. Frontend gets a `<input type="date">` that appears under the "С этого урока" radio in `SeriesDialog`.

**Tech Stack:** Go (Gin, pgx/v5), React/Next.js, TypeScript, TanStack Query

---

## File Map

| File | Change |
|------|--------|
| `repository/lesson.go` | Add `toDate *string` to `LessonRepository` interface and `lessonRepository.DeleteSeries` |
| `service/lesson.go` | Add `toDate *string` to `LessonService` interface and `lessonService.DeleteSeries` |
| `service/lesson_test.go` | Update `mockLessonRepo.DeleteSeries` signature; add `TestDeleteSeries_WithToDate` tests |
| `handlers/mocks_test.go` | Update `mockLessonRepo.DeleteSeries` and `mockLessonService.DeleteSeries` signatures |
| `frontend/src/lib/api/lessons.ts` | Add `toDate?: string` to `deleteSeries` |
| `frontend/src/lib/hooks/useLessons.ts` | Add `toDate?: string` to `useDeleteSeries` mutation variables |
| `frontend/src/components/lessons/SeriesDialog.tsx` | Add `toDate` state + datepicker; update `onDelete` prop type |
| `frontend/src/app/(dashboard)/courses/[id]/page.tsx` | Update `handleSeriesDelete` to pass `toDate` |

---

### Task 1: Repository — add `toDate` to `DeleteSeries`

**Files:**
- Modify: `repository/lesson.go`

- [ ] **Step 1: Update the interface**

In `repository/lesson.go`, change line 26:
```go
// Before:
DeleteSeries(ctx context.Context, seriesID string, tutorID string, fromDate *string) error
// After:
DeleteSeries(ctx context.Context, seriesID string, tutorID string, fromDate *string, toDate *string) error
```

- [ ] **Step 2: Update the implementation**

Replace the `lessonRepository.DeleteSeries` method (currently lines 182–200) with:
```go
func (r *lessonRepository) DeleteSeries(ctx context.Context, seriesID string, tutorID string, fromDate *string, toDate *string) error {
	args := []interface{}{seriesID, tutorID}
	fromClause := ""
	toClause := ""
	if fromDate != nil {
		fromClause = fmt.Sprintf("AND lessons.scheduled_at >= $%d::timestamptz", len(args)+1)
		args = append(args, *fromDate)
	}
	if toDate != nil {
		toClause = fmt.Sprintf("AND lessons.scheduled_at <= $%d::timestamptz", len(args)+1)
		args = append(args, *toDate)
	}

	query := fmt.Sprintf(`
		DELETE FROM lessons
		USING courses
		WHERE lessons.series_id = $1
		  AND lessons.course_id = courses.id
		  AND courses.tutor_id = $2
		  %s
		  %s`, fromClause, toClause)

	_, err := r.pool.Exec(ctx, query, args...)
	return err
}
```

- [ ] **Step 3: Verify it compiles**

```bash
go build ./...
```

Expected: no output (clean build). Fix any compile errors before continuing.

---

### Task 2: Service — add `toDate` to `DeleteSeries`

**Files:**
- Modify: `service/lesson.go`
- Modify: `service/lesson_test.go`

- [ ] **Step 1: Update mock in `service/lesson_test.go`**

Find `mockLessonRepo.DeleteSeries` (currently: `func (m *mockLessonRepo) DeleteSeries(ctx context.Context, seriesID string, tutorID string, fromDate *string) error`) and update its signature:
```go
func (m *mockLessonRepo) DeleteSeries(ctx context.Context, seriesID string, tutorID string, fromDate *string, toDate *string) error {
	return m.Called(ctx, seriesID, tutorID, fromDate, toDate).Error(0)
}
```

- [ ] **Step 2: Add failing tests at the bottom of `service/lesson_test.go`**

```go
// DeleteSeries

func TestDeleteSeries_NoRange(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	lessonRepo.On("DeleteSeries", mock.Anything, "series-1", tutorID, (*string)(nil), (*string)(nil)).Return(nil)

	err := svc.DeleteSeries(context.Background(), "series-1", tutorID, nil, nil)

	assert.NoError(t, err)
	lessonRepo.AssertExpectations(t)
}

func TestDeleteSeries_WithFromAndTo(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	from := "2026-06-05T00:00:00Z"
	to   := "2026-12-31T23:59:59Z"
	lessonRepo.On("DeleteSeries", mock.Anything, "series-1", tutorID, &from, &to).Return(nil)

	err := svc.DeleteSeries(context.Background(), "series-1", tutorID, &from, &to)

	assert.NoError(t, err)
	lessonRepo.AssertExpectations(t)
}

func TestDeleteSeries_RepoError(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	lessonRepo.On("DeleteSeries", mock.Anything, "series-1", tutorID, (*string)(nil), (*string)(nil)).Return(errors.New("db error"))

	err := svc.DeleteSeries(context.Background(), "series-1", tutorID, nil, nil)

	assert.Error(t, err)
	lessonRepo.AssertExpectations(t)
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./service/ -run TestDeleteSeries -v
```

Expected: compilation error — `LessonService.DeleteSeries` and `lessonService.DeleteSeries` still have old signature.

- [ ] **Step 4: Update `LessonService` interface in `service/lesson.go`**

Change line 20:
```go
// Before:
DeleteSeries(ctx context.Context, seriesID string, tutorID string, fromDate *string) error
// After:
DeleteSeries(ctx context.Context, seriesID string, tutorID string, fromDate *string, toDate *string) error
```

- [ ] **Step 5: Update `lessonService.DeleteSeries` implementation**

Find the `DeleteSeries` method in `service/lesson.go` and update it:
```go
func (s *lessonService) DeleteSeries(ctx context.Context, seriesID string, tutorID string, fromDate *string, toDate *string) error {
	return s.repo.DeleteSeries(ctx, seriesID, tutorID, fromDate, toDate)
}
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
go test ./service/ -run TestDeleteSeries -v
```

Expected:
```
--- PASS: TestDeleteSeries_NoRange
--- PASS: TestDeleteSeries_WithFromAndTo
--- PASS: TestDeleteSeries_RepoError
PASS
```

- [ ] **Step 7: Run all service tests to check for regressions**

```bash
go test ./service/... -v 2>&1 | tail -20
```

Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
git add repository/lesson.go service/lesson.go service/lesson_test.go
git commit -m "feat: add toDate upper bound to DeleteSeries"
```

---

### Task 3: Handler — read `to` query param

**Files:**
- Modify: `handlers/mocks_test.go`
- Modify: `handlers/lesson.go`

- [ ] **Step 1: Update both mocks in `handlers/mocks_test.go`**

Find `mockLessonRepo.DeleteSeries` and update:
```go
func (m *mockLessonRepo) DeleteSeries(ctx context.Context, seriesID string, tutorID string, fromDate *string, toDate *string) error {
	return m.Called(ctx, seriesID, tutorID, fromDate, toDate).Error(0)
}
```

Find `mockLessonService.DeleteSeries` and update:
```go
func (m *mockLessonService) DeleteSeries(ctx context.Context, seriesID string, tutorID string, fromDate *string, toDate *string) error {
	return m.Called(ctx, seriesID, tutorID, fromDate, toDate).Error(0)
}
```

- [ ] **Step 2: Update `DeleteSeries` handler in `handlers/lesson.go`**

Replace the `DeleteSeries` method (currently lines 195–214):
```go
func (h *LessonHandler) DeleteSeries(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	seriesID := c.Param("seriesId")

	var fromDatePtr, toDatePtr *string
	if from := c.Query("from"); from != "" {
		fromDatePtr = &from
	}
	if to := c.Query("to"); to != "" {
		toDatePtr = &to
	}

	if err := h.service.DeleteSeries(c.Request.Context(), seriesID, tutorID, fromDatePtr, toDatePtr); err != nil {
		h.log.Error("Failed to delete series", slog.String("seriesId", seriesID), slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	h.log.Info("Series deleted", slog.String("seriesId", seriesID))
	c.Status(http.StatusNoContent)
}
```

- [ ] **Step 3: Build and run all tests**

```bash
go build ./... && go test ./...
```

Expected: all PASS, no compilation errors.

- [ ] **Step 4: Commit**

```bash
git add handlers/lesson.go handlers/mocks_test.go
git commit -m "feat: handler reads 'to' query param for series deletion"
```

---

### Task 4: Frontend API and hook

**Files:**
- Modify: `frontend/src/lib/api/lessons.ts`
- Modify: `frontend/src/lib/hooks/useLessons.ts`

- [ ] **Step 1: Update `deleteSeries` in `frontend/src/lib/api/lessons.ts`**

Replace line 53–54:
```ts
deleteSeries: (seriesId: string, fromDate?: string, toDate?: string) =>
  api.delete(`/lessons/series/${seriesId}`, {
    params: {
      ...(fromDate && { from: fromDate }),
      ...(toDate  && { to:   toDate  }),
    },
  }).then(() => undefined),
```

- [ ] **Step 2: Update `useDeleteSeries` in `frontend/src/lib/hooks/useLessons.ts`**

Replace lines 93–100:
```ts
export function useDeleteSeries(courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ seriesId, fromDate, toDate }: { seriesId: string; fromDate?: string; toDate?: string }) =>
      lessonsApi.deleteSeries(seriesId, fromDate, toDate),
    onSuccess: () => qc.invalidateQueries({ queryKey: lessonKeys.byCourse(courseId) }),
  })
}
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd frontend && npx tsc --noEmit 2>&1 | grep -E "error TS" | head -20
```

Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/api/lessons.ts frontend/src/lib/hooks/useLessons.ts
git commit -m "feat: pass toDate to deleteSeries API"
```

---

### Task 5: SeriesDialog — add "to date" picker

**Files:**
- Modify: `frontend/src/components/lessons/SeriesDialog.tsx`

- [ ] **Step 1: Update the `SeriesDialogProps` interface and `onDelete` signature**

Change line 17:
```ts
onDelete: (seriesId: string, fromDate?: string, toDate?: string) => Promise<void>
```

- [ ] **Step 2: Add `toDate` state and reset**

Add after line 29 (`const [deleting, setDeleting] = useState(false)`):
```ts
const [toDate, setToDate] = useState('')
```

In the `useEffect` reset block (after `setNotes('')`), add:
```ts
setToDate('')
```

- [ ] **Step 3: Build the `toDate` ISO string for the API call**

Add this helper after the `fromDate` declaration (line 41):
```ts
const toDateISO = toDate ? `${toDate}T23:59:59Z` : undefined
```

- [ ] **Step 4: Pass `toDateISO` in `handleDelete`**

Change line 84:
```ts
await onDelete(lesson.series_id!, fromDate, toDateISO)
```

- [ ] **Step 5: Add date input to the JSX**

After the "С этого урока" radio label block (after the closing `</label>` of scope `'from'`), add a conditional date picker. The full updated radio section (replace the `<div className="space-y-1.5">` block that starts at line 103 with):

```tsx
<div className="space-y-1.5">
  <Label>Применить к</Label>
  <div className="space-y-1.5">
    <label className="flex items-center gap-2 text-sm cursor-pointer">
      <input type="radio" name="scope" value="all"
        checked={scope === 'all'} onChange={() => setScope('all')} />
      Все уроки серии
    </label>
    <label className="flex items-center gap-2 text-sm cursor-pointer">
      <input type="radio" name="scope" value="from"
        checked={scope === 'from'} onChange={() => setScope('from')} />
      С этого урока ({lessonDate})
    </label>
    {scope === 'from' && (
      <div className="pl-6 space-y-1">
        <Label className="text-xs text-muted-foreground">По дату (необязательно)</Label>
        <input
          type="date"
          value={toDate}
          onChange={(e) => setToDate(e.target.value)}
          className="block w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm"
        />
      </div>
    )}
  </div>
</div>
```

- [ ] **Step 6: Verify TypeScript compiles**

```bash
cd frontend && npx tsc --noEmit 2>&1 | grep -E "error TS" | head -20
```

Expected: no output.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/components/lessons/SeriesDialog.tsx
git commit -m "feat: add optional end-date picker to SeriesDialog"
```

---

### Task 6: Course page — wire `toDate`

**Files:**
- Modify: `frontend/src/app/(dashboard)/courses/[id]/page.tsx`

- [ ] **Step 1: Update `handleSeriesDelete` signature and body**

Find `handleSeriesDelete` (around line 223) and update:
```ts
async function handleSeriesDelete(seriesId: string, fromDate?: string, toDate?: string) {
  await deleteSeries.mutateAsync({ seriesId, fromDate, toDate })
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd frontend && npx tsc --noEmit 2>&1 | grep -E "error TS" | head -20
```

Expected: no output.

- [ ] **Step 3: Run full build to check for regressions**

```bash
cd frontend && npm run build 2>&1 | tail -20
```

Expected: `✓ Compiled successfully` or similar, no type errors.

- [ ] **Step 4: Commit**

```bash
git add "frontend/src/app/(dashboard)/courses/[id]/page.tsx"
git commit -m "feat: wire toDate through course page series delete handler"
```

---

## Manual Verification

1. Start the app: `air` (backend) + `cd frontend && npm run dev` (frontend)
2. Open a course with a series that has future lessons
3. Click a lesson that is in the middle of the series
4. Open series management dialog (click the series icon/button)
5. Select "С этого урока" — verify the date picker appears below
6. Fill in an end date a few weeks out → click "Удалить серию"
7. Confirm that only lessons in the `[lesson.scheduled_at, toDate]` range were deleted; past lessons remain
8. Repeat without filling the end date — verify all lessons from this one to the end are deleted (original behaviour preserved)
