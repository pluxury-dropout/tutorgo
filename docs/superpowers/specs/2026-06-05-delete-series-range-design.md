# Delete Series by Date Range

**Date:** 2026-06-05  
**Status:** Approved

## Problem

Tutors create lessons in bulk (series) for the whole year. When a course ends mid-series, the tutor needs to delete only future lessons while keeping past ones for statistics. The current API supports `from` (lower bound) but not `to` (upper bound), so there is no way to delete a range like "from today to December 31".

## Solution

Extend `DeleteSeries` across all layers to accept an optional `toDate` upper bound. No new endpoints or migrations needed.

## Backend

### Repository (`repository/lesson.go`)

`DeleteSeries(ctx, seriesID, tutorID, fromDate *string, toDate *string) error`

SQL condition added when `toDate` is non-nil:
```sql
AND lessons.scheduled_at <= $N::timestamptz
```

Upper bound is **inclusive** — lessons scheduled exactly on `toDate` are deleted.

### Service (`service/lesson.go`)

`DeleteSeries(ctx, seriesID, tutorID string, fromDate, toDate *string) error`

Passes both params through to the repository unchanged. No business logic — the repository owns the SQL.

### Handler (`handlers/lesson.go`)

Reads `to` query parameter:
```
DELETE /lessons/series/:seriesId?from=2026-06-05T00:00:00Z&to=2026-12-31T23:59:59Z
```

Both `from` and `to` are optional independently. Without `to`, behaviour is identical to today.

### Interface

`LessonRepository.DeleteSeries` signature updated to match.

## Frontend

### `SeriesDialog.tsx`

When scope `'from'` is selected, a date input appears below the radio:

```
○ Все уроки серии
● С этого урока (5 июня, 10:00)
    По дату (необязательно): [__________]
```

- Input type: `date` (browser native picker)
- If empty: `toDate` is not sent → deletes to the end of series (existing behaviour)
- If filled: the selected date is sent as `toDate` in ISO format with time `T23:59:59Z` to be inclusive of that day

`onDelete` signature extended:
```ts
onDelete: (seriesId: string, fromDate?: string, toDate?: string) => Promise<void>
```

### `lessons.ts`

```ts
deleteSeries: (seriesId: string, fromDate?: string, toDate?: string) =>
  api.delete(`/lessons/series/${seriesId}`, {
    params: { ...(fromDate && { from: fromDate }), ...(toDate && { to: toDate }) }
  }).then(() => undefined)
```

### `courses/[id]/page.tsx`

`handleSeriesDelete` updated to accept and forward `toDate`:
```ts
async function handleSeriesDelete(seriesId: string, fromDate?: string, toDate?: string) {
  await deleteSeries.mutateAsync({ seriesId, fromDate, toDate })
}
```

`useLessons` hook's `deleteSeriesMutation` updated similarly.

## Edge Cases

- `toDate` without `fromDate`: valid — deletes lessons up to a date from the beginning of the series
- `fromDate > toDate`: results in zero deletions (SQL range is empty), no error returned
- Both absent: deletes the entire series (unchanged behaviour)

## Testing

- Service-layer unit test: mock repo called with correct `fromDate`/`toDate` pointers
- Manual: open SeriesDialog, select "С этого урока", fill end date, confirm deletion removes only the expected range
