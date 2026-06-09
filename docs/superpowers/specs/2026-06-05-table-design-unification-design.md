# Table Design Unification — Students & Courses Pages

**Date:** 2026-06-05  
**Status:** Approved  
**Scope:** `/students/page.tsx`, `/courses/[id]/page.tsx` (courses list only)

## Goal

Make the table/list sections on the Students and Courses pages visually match the dashboard widget style: no rounded card container, hairline borders between rows, div-based CSS Grid layout, inline `style={{}}` with CSS variables.

## What Changes

Only the table section of each page. PageHeader, search input, loading skeleton, EmptyState, pagination, and form dialogs are unchanged.

## Design

### Removed

- `<div className="border rounded-lg overflow-hidden">` outer wrapper
- `<table>`, `<thead>`, `<tbody>`, `<tr>`, `<th>`, `<td>` HTML elements
- `bg-muted/40` background on the header row
- Tailwind border/spacing classes on rows

### Added

**Column header row** — a single `div` with CSS Grid matching the data row grid:
- `fontSize: 11.5, fontWeight: 500, color: 'var(--muted-foreground)', letterSpacing: '0.05em', textTransform: 'uppercase'`
- `paddingBottom: 8, borderBottom: '1px solid var(--border)'`

**Data rows** — `div` with CSS Grid:
- `borderTop: '1px solid var(--border)'` on all rows except the first (`isFirst` flag)
- `padding: '8px 0'`
- `cursor: 'pointer'` on clickable rows
- Hover: `background: 'var(--muted)'` (inline via `onMouseEnter/onMouseLeave`)
- Main cell (name/subject): `fontSize: 14, fontWeight: 600, color: 'var(--foreground)'`
- Secondary cells: `fontSize: 13, color: 'var(--muted-foreground)'`

### Grid Columns

**Students** (`gridTemplateColumns: '1fr 1fr 1fr auto'`):
- Имя | Email | Телефон | Действия

**Courses** (`gridTemplateColumns: '1.5fr 80px 1fr 160px 90px auto'`):
- Предмет | Тип | Ученик | Цена за цикл | Начало | Действия

### Hover State

Since rows are `div` elements (not `<tr>`), hover is handled via `onMouseEnter` / `onMouseLeave` with local state or inline handlers setting `background`. Use a `hoveredId` state variable per page.

### Actions Column

Edit/Delete buttons remain in the last column. ChevronRight icon for navigate-on-click rows is removed (replaced by cursor pointer affordance on the row itself) — or kept as a span absolutely positioned at the right edge if desired. Keep it for consistency with current behavior.

Actually: keep ChevronRight as a small icon in a dedicated column, same as current behavior.

**Students** grid: `1fr 1fr 1fr 16px auto`  
**Courses** grid: `1.5fr 80px 1fr 160px 90px 16px auto`

The `16px` column holds `ChevronRight`, shown on hover via `opacity: hoveredId === id ? 1 : 0`.

## Files Affected

- `src/app/(dashboard)/students/page.tsx` — replace table section
- `src/app/(dashboard)/courses/page.tsx` — replace table section

## Out of Scope

- `courses/[id]/page.tsx` (course detail page — has its own lesson table, separate task)
- `payments/page.tsx` — separate decision
- PageHeader, StudentForm, CourseForm components
