# Handoff: TutorGo Frontend Redesign

## Overview
TutorGo is a web app for freelance private tutors to manage their teaching business — students, courses, scheduling, payments, and attendance. This handoff covers the complete UI/UX redesign of all six primary screens.

## About the Design Files
The files in this bundle (`TutorGo.html`, `tweaks-panel.jsx`) are **design references created in HTML** — high-fidelity interactive prototypes showing intended look, layout, and behavior. They are **not production code to copy directly**.

Your task is to **recreate these designs in the existing Next.js 16 (App Router) + TypeScript + Tailwind v4 + shadcn/ui codebase**, using its established patterns and libraries. Map every visual element and interaction to your stack's equivalents (e.g. shadcn `Card`, `Button`, `Dialog`, `Badge`, `Input`; FullCalendar for the calendar screen).

## Fidelity
**High-fidelity.** These are pixel-complete mockups with final colors, typography, spacing, interactions, and copy. Recreate them precisely using your codebase's existing component library. All text is in Russian.

---

## Design Tokens

All values use OKLCH. Add these to your global CSS as CSS custom properties:

```css
:root {
  /* Base */
  --background:            oklch(0.985 0.004 210);
  --foreground:            oklch(0.18 0.02 215);
  --primary:               oklch(0.52 0.1 185);   /* muted teal — brand color */
  --primary-light:         oklch(0.94 0.04 185);
  --primary-foreground:    oklch(0.985 0 0);
  --secondary:             oklch(0.96 0.004 215);
  --muted:                 oklch(0.96 0.004 215);
  --muted-foreground:      oklch(0.50 0.02 215);
  --accent:                oklch(0.72 0.14 55);   /* warm amber */
  --accent-light:          oklch(0.96 0.04 55);
  --border:                oklch(0.91 0.006 215);
  --radius:                0.5rem;
  --radius-lg:             0.75rem;

  /* Sidebar */
  --sidebar-active-bg:     oklch(0.94 0.04 185);
  --sidebar-active-text:   oklch(0.52 0.1 185);
  --sidebar-hover-bg:      oklch(0.96 0.004 215);
  --sidebar-text:          oklch(0.42 0.02 215);

  /* Lesson status chips */
  --status-scheduled-bg:   oklch(0.94 0.04 250);
  --status-scheduled-text: oklch(0.40 0.12 250);
  --status-completed-bg:   oklch(0.93 0.06 155);
  --status-completed-text: oklch(0.35 0.10 155);
  --status-cancelled-bg:   oklch(0.94 0.004 215);
  --status-cancelled-text: oklch(0.48 0.01 215);
  --status-missed-bg:      oklch(0.94 0.04 25);
  --status-missed-text:    oklch(0.42 0.18 25);

  /* Calendar event chips */
  --cal-scheduled-bg:      oklch(0.92 0.05 250);
  --cal-completed-bg:      oklch(0.91 0.07 155);
  --cal-cancelled-bg:      oklch(0.93 0.003 215);
  --cal-missed-bg:         oklch(0.92 0.05 25);
}
```

### Typography
- **Font**: Plus Jakarta Sans (Google Fonts), weights 400 / 500 / 600 / 700
- **Base size**: 14px
- **Page titles**: 22px / 700 / letter-spacing -0.4px
- **Card titles**: 14px / 600
- **Table headers**: 11.5px / 600 / letter-spacing 0.3px / uppercase in small labels
- **Muted labels**: 12–13px / 500 / `--muted-foreground`

### Spacing
- **Page padding**: 28px vertical / 32px horizontal
- **Content gap**: 24px
- **Card header padding**: 18px 20px
- **Row padding**: 13px 20px
- **Stat card padding**: 20px

### Shadows & Surfaces
- **Cards**: `0 1px 3px oklch(0.18 0.02 215 / 0.08)` — subtle only
- **Sidebar**: same background as page (`--background`), no elevated shadow
- **Border on cards**: `1px solid var(--border)`

---

## Layout

### App Shell
```
┌──────────────────────────────────────────────────┐
│  Sidebar (240px fixed)  │  Main content (flex:1)  │
│  - Logo + wordmark      │  - Page header          │
│  - Nav items (6)        │  - Page content         │
│  - User avatar/name     │                         │
└──────────────────────────────────────────────────┘
```

**Sidebar:**
- Fixed left, full height, `border-right: 1px solid var(--border)`
- Same background as page (no contrast)
- Nav items: icon (17px) + label, 9px 12px padding, 0.5rem border-radius
- **Active state**: `background: var(--sidebar-active-bg)`, `color: var(--sidebar-active-text)`, font-weight 600
- **Hover state**: `background: var(--sidebar-hover-bg)`, color foreground
- Footer: 34px avatar circle (initials) + tutor name + role label

---

## Screens

### 1. Dashboard (`/`)
**Purpose**: Daily overview — today's lessons + recent payments + stat cards.

**Layout**: Page header → 4-column stat grid → 2-column content (lessons list | payments list)

**Stat cards** (4 columns, gap 14px):
- White background, `border: 1px solid var(--border)`, `border-radius: var(--radius-lg)`
- Each card: colored icon square (36×36px, 9px radius) + label (12px muted) + large value (26px/700) + delta note (12px)
- Icon square colors: teal (`--primary-light`), amber (`--accent-light`), purple (`oklch(0.94 0.03 280)`), sage (`oklch(0.92 0.05 155)`)
- Stats: **Уроков сегодня**, **Доход за месяц (₸)**, **Активных учеников**, **Активных курсов**

**Today's Lessons card** (left, flex:1):
- Card with header "Уроки сегодня" + link "Расписание →"
- Each row: `time (46px min-width) · colored dot (8px circle) · subject + student/duration · status chip · optional action button`
- Row padding: 13px 20px, border-bottom between rows, hover: `--secondary` bg

**Recent Payments card** (right, 380px):
- Card with header "Последние платежи" + link "Все →"
- Each row: `avatar circle (32px, initials) · name + course/count · amount (₸) + date`
- Last 5 payments

### 2. Students (`/students`)
**Purpose**: Browse and search all students.

**Layout**: Page header with "+ Новый ученик" button → search bar (280px) → full-width table card

**Table columns**: Avatar + Name/email | Phone | Active courses | Remaining lessons balance | Action button

**Notes**:
- Avatar: 34px circle, `--primary-light` bg, primary color initials, font-weight 700
- Remaining lessons: colored by urgency (low = `--status-missed-text`)
- Row hover: `oklch(0.975 0.004 210)` bg

### 3. Courses (`/courses`)
**Purpose**: Browse all courses with type filter.

**Layout**: Page header → filter tabs (All / Individual / Group) → full-width table card

**Filter tabs**: Segmented control, active tab has white bg + shadow, inactive is muted bg

**Table columns**: Subject | Type badge | Student / Group | Price/lesson (₸) | Start date | Action button

**Type badges**:
- Individual: `background: var(--primary-light)`, `color: var(--primary)`, pill shape
- Group: `background: var(--accent-light)`, `color: oklch(0.48 0.18 55)`

### 4. Calendar (`/calendar`) — FullCalendar
**Purpose**: Weekly/monthly/daily schedule. The most interactive screen.

**Tech**: Use FullCalendar (`@fullcalendar/react`) with `timeGridWeek`, `timeGridDay`, `dayGridMonth` views.

**Header controls**:
- Back/forward: icon-only chevron buttons (32×32px, border, hover: `--secondary`)
- "Сегодня" button: same size, `color: var(--primary)`, `border-color: var(--primary)`
- View switcher: segmented tabs (Месяц / Неделя / День)

**Week/Day view styling**:
- Default view: `timeGridWeek`
- Time column: 52px wide, hour labels right-aligned, 11px / muted
- Slot height: 72px per hour
- Today column: subtle teal tint `oklch(0.97 0.015 185)`
- Current time: red dot + 2px line spanning today's column only
- Empty slot hover: `oklch(0.95 0.04 185 / 0.3)`

**Event chips** (all 4 statuses — flat pastel, no left border accent):
| Status | Background | Text color |
|--------|-----------|------------|
| scheduled | `oklch(0.92 0.05 250)` | `oklch(0.35 0.14 250)` |
| completed | `oklch(0.91 0.07 155)` | `oklch(0.32 0.10 155)` |
| cancelled | `oklch(0.93 0.003 215)` | `oklch(0.45 0.01 215)` |
| missed | `oklch(0.92 0.05 25)` | `oklch(0.38 0.18 25)` |

**Chip content**: time range (10px) + subject (11px/600) + student name (10px/400)

**Interactions**:
- **Drag to reschedule**: FullCalendar `editable: true` — enables drag-drop. On drop, snap to 30-min grid. Use spring easing on the snap animation (`cubic-bezier(0.34, 1.56, 0.64, 1)`, 180ms).
- **Resize duration**: FullCalendar `editable: true` also enables bottom-edge resize. Snap to 15-min grid.
- **Click event**: navigate to `/courses/[id]`
- **Click empty slot**: open "New lesson" dialog

**Month view**:
- Standard `dayGridMonth`
- Date number centered at top of each cell
- Pastel event chips (same colors as above), truncated with ellipsis if too many
- Today's date: circle with `var(--primary)` background
- Other-month dates: `--muted-foreground`

**Day view**:
- `timeGridDay` — single column, white background (no today-tint on slots)
- Large date circle header (48×48px, `var(--primary)` bg) + day name + date label

### 5. Payments (`/payments`)
**Purpose**: Payment history + earnings overview.

**Layout**: 3-column stat cards → sortable table

**Stat cards** (3 columns): All time / Last month / This month — each shows total ₸ + count

**Table columns**: Date | Student | Course | Lessons count | Amount (₸, right-aligned, 700 weight)

**Click row**: navigate to `/courses/[id]`

### 6. Profile (`/profile`)
**Purpose**: Tutor personal info + password change.

**Layout**: 2-column grid

**Left card** — "Личные данные": avatar (56px) + name/role → 2-column form grid (Имя, Фамилия, Email, Телефон) → "Редактировать" button opens inline editing

**Right card** — "Смена пароля": 3 password inputs + "Сохранить пароль" primary button

---

## Component Specs

### Status Chip
```
padding: 3px 9px
border-radius: 20px
font-size: 12px / weight 600
display: inline-flex, align-items: center
```
Use the 4 status token pairs from Design Tokens above.

### Lesson Row
```
display: flex, align-items: center, gap: 14px
padding: 13px 20px
border-bottom: 1px solid var(--border)
hover: background var(--secondary)
```
Contents: time label (46px min-width, 600, muted) · 8px dot (colored by status) · subject (600) + student (12px muted) · status chip · optional action button

### Payment Row
```
display: flex, align-items: center, gap: 12px
padding: 12px 20px
```
Contents: 32px avatar circle · name (600) + course/lessons (12px muted) · amount (700, 14px) + date (11px muted, right-aligned)

### Stat Card
```
background: white
border: 1px solid var(--border)
border-radius: var(--radius-lg)
padding: 20px
box-shadow: 0 1px 3px oklch(0.18 0.02 215 / 0.08)
```
Contents: icon square → label (12px/500 muted) → value (26px/700) → delta note (12px)

---

## Interactions & Behavior

| Interaction | Behavior |
|-------------|----------|
| Sidebar nav click | Instant route change, active state updates |
| Student row click | Navigate to `/students/[id]` |
| Course row click | Navigate to `/courses/[id]` |
| Payment row click | Navigate to `/courses/[id]` |
| Calendar event click | Navigate to `/courses/[id]` |
| Calendar empty slot click | Open "New lesson" dialog |
| Drag lesson | Reschedule (FullCalendar editable) — 30min snap |
| Resize lesson | Change duration (FullCalendar editable) — 15min snap |
| Drop/snap animation | `cubic-bezier(0.34, 1.56, 0.64, 1)`, 180ms on `left` + `top` |

---

## Assets & Icons
- Icons: simple SVG line icons, 17×17px, `stroke-width: 2`, `stroke-linecap: round`
- No icon library required — all used icons are minimal geometric shapes (see sidebar in HTML for SVG source)
- No images used in the design — avatar initials only

---

## Files in This Package

| File | Purpose |
|------|---------|
| `TutorGo.html` | Complete interactive prototype — all 6 screens, drag/resize calendar, tweaks panel |
| `tweaks-panel.jsx` | Tweaks panel component (design exploration tool only — not for production) |

Open `TutorGo.html` in any browser to explore the full prototype interactively. Use browser DevTools to inspect exact computed values for any element.

---

## Implementation Notes for Claude Code

1. **Start with the design tokens** — add the CSS variables to your `globals.css` or Tailwind config
2. **Sidebar first** — it wraps every page; implement it as a shared layout component
3. **Dashboard** — use shadcn `Card` for stat cards and content cards
4. **Calendar** — configure FullCalendar with `editable: true`, custom event rendering using the status color tokens, and the `timeGridWeek` default view. Style the today column tint via FullCalendar's `dayCellClassNames` / `slotLaneClassNames` hooks
5. **Status chips** — create a shared `<LessonStatusBadge status="scheduled|completed|cancelled|missed" />` component used across all screens
6. **Currency formatting** — all amounts in KZT tenge (₸), formatted as `₸X XXX` (space as thousands separator)
7. **All UI copy is in Russian** — do not translate
