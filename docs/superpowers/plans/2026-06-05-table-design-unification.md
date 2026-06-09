# Table Design Unification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the rounded-card HTML table on Students and Courses list pages with dashboard-style div+CSS Grid rows and hairline borders.

**Architecture:** Two independent file edits — one per page. Each replaces only the table/list section (inside the `students.length > 0` branch). PageHeader, search, skeleton, EmptyState, pagination, and form dialogs are untouched. Rows use inline `style={{}}` with CSS variables for borders and colours; Tailwind `className` is kept only for hover (`hover:bg-muted/30`) and the `group` utility for ChevronRight fade.

**Tech Stack:** Next.js (App Router), React, Tailwind CSS, CSS custom properties (`var(--border)`, `var(--foreground)`, `var(--muted-foreground)`)

---

### Task 1: Refactor Students list to dashboard-style

**Files:**
- Modify: `frontend/src/app/(dashboard)/students/page.tsx`

- [ ] **Step 1: Locate the table section**

In `students/page.tsx`, find the block that starts at:
```tsx
<div className="border rounded-lg overflow-hidden">
  <table className="w-full text-sm">
```
This is the only block to replace. Everything outside it (PageHeader, search Input, skeleton, EmptyState, pagination, StudentForm) stays unchanged.

- [ ] **Step 2: Replace the table block**

Replace the entire `<div className="border rounded-lg overflow-hidden">…</div>` block with:

```tsx
<div>
  {/* Column headers */}
  <div style={{
    display: 'grid',
    gridTemplateColumns: '1fr 1fr 1fr 16px auto',
    alignItems: 'baseline',
    gap: 12,
    paddingBottom: 8,
    borderBottom: '1px solid var(--border)',
  }}>
    {['Имя', 'Email', 'Телефон', '', ''].map((label, i) => (
      <span key={i} style={{
        fontSize: 11.5, fontWeight: 500,
        color: 'var(--muted-foreground)',
        letterSpacing: '0.05em',
        textTransform: 'uppercase',
      }}>{label}</span>
    ))}
  </div>
  {/* Rows */}
  {students.map((student, i) => (
    <div
      key={student.id}
      style={{
        display: 'grid',
        gridTemplateColumns: '1fr 1fr 1fr 16px auto',
        alignItems: 'center',
        gap: 12,
        padding: '8px 0',
        borderTop: i === 0 ? 'none' : '1px solid var(--border)',
        cursor: 'pointer',
      }}
      className="hover:bg-muted/30 group"
      onClick={() => router.push(`/students/${student.id}`)}
    >
      <span style={{
        fontSize: 14, fontWeight: 600, color: 'var(--foreground)',
        overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
      }}>
        {student.first_name}{student.last_name ? ` ${student.last_name}` : ''}
      </span>
      <span style={{ fontSize: 13, color: 'var(--muted-foreground)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
        {student.email}
      </span>
      <span style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>
        {student.phone || '—'}
      </span>
      <ChevronRight className="h-4 w-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity duration-150" />
      <div className="flex items-center justify-end gap-1" onClick={(e) => e.stopPropagation()}>
        <Button size="icon" variant="ghost" className="h-8 w-8" onClick={() => openEdit(student)}>
          <Pencil className="h-3.5 w-3.5" />
        </Button>
        <Button size="icon" variant="ghost"
          className="h-8 w-8 text-destructive hover:text-destructive"
          onClick={() => handleDelete(student)}>
          <Trash2 className="h-3.5 w-3.5" />
        </Button>
      </div>
    </div>
  ))}
</div>
```

- [ ] **Step 3: Check TypeScript compiles**

```bash
cd frontend && npx tsc --noEmit 2>&1 | head -20
```
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/app/\(dashboard\)/students/page.tsx
git commit -m "feat: replace students table with dashboard-style list"
```

---

### Task 2: Refactor Courses list to dashboard-style

**Files:**
- Modify: `frontend/src/app/(dashboard)/courses/page.tsx`

- [ ] **Step 1: Locate the table section**

In `courses/page.tsx`, find the block that starts at:
```tsx
<div className="border rounded-lg overflow-hidden">
  <table className="w-full text-sm">
```
Only this block is replaced. Everything outside it stays unchanged.

- [ ] **Step 2: Replace the table block**

Replace the entire `<div className="border rounded-lg overflow-hidden">…</div>` block with:

```tsx
<div>
  {/* Column headers */}
  <div style={{
    display: 'grid',
    gridTemplateColumns: '1.5fr 80px 1fr 160px 90px 16px auto',
    alignItems: 'baseline',
    gap: 12,
    paddingBottom: 8,
    borderBottom: '1px solid var(--border)',
  }}>
    {['Предмет', 'Тип', 'Ученик', 'Цена за цикл', 'Начало', '', ''].map((label, i) => (
      <span key={i} style={{
        fontSize: 11.5, fontWeight: 500,
        color: 'var(--muted-foreground)',
        letterSpacing: '0.05em',
        textTransform: 'uppercase',
      }}>{label}</span>
    ))}
  </div>
  {/* Rows */}
  {courses.map((course, i) => (
    <div
      key={course.id}
      style={{
        display: 'grid',
        gridTemplateColumns: '1.5fr 80px 1fr 160px 90px 16px auto',
        alignItems: 'center',
        gap: 12,
        padding: '8px 0',
        borderTop: i === 0 ? 'none' : '1px solid var(--border)',
        cursor: 'pointer',
      }}
      className="hover:bg-muted/30 group"
      onClick={() => router.push(`/courses/${course.id}`)}
    >
      <span style={{
        fontSize: 14, fontWeight: 600, color: 'var(--foreground)',
        overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
      }}>
        {course.subject}
      </span>
      <span><CourseTypeBadge isGroup={!course.student_id} /></span>
      <span style={{ fontSize: 13, color: 'var(--muted-foreground)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
        {studentName(course) ?? '—'}
      </span>
      <span style={{ fontSize: 13, color: 'var(--muted-foreground)', fontVariantNumeric: 'tabular-nums' }}>
        {course.price_per_cycle.toLocaleString()} ₸ / {course.lessons_per_cycle} ур.
      </span>
      <span style={{ fontSize: 13, color: 'var(--muted-foreground)', fontVariantNumeric: 'tabular-nums' }}>
        {new Date(course.started_at).toLocaleDateString('ru-RU')}
      </span>
      <ChevronRight className="h-4 w-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity duration-150" />
      <div className="flex items-center justify-end gap-1" onClick={(e) => e.stopPropagation()}>
        <Button size="icon" variant="ghost" className="h-8 w-8" onClick={() => openEdit(course)}>
          <Pencil className="h-3.5 w-3.5" />
        </Button>
        <Button size="icon" variant="ghost"
          className="h-8 w-8 text-destructive hover:text-destructive"
          onClick={() => handleDelete(course)}>
          <Trash2 className="h-3.5 w-3.5" />
        </Button>
      </div>
    </div>
  ))}
</div>
```

- [ ] **Step 3: Check TypeScript compiles**

```bash
cd frontend && npx tsc --noEmit 2>&1 | head -20
```
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/app/\(dashboard\)/courses/page.tsx
git commit -m "feat: replace courses table with dashboard-style list"
```
