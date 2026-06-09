# Color Palette Dev Page

**Date:** 2026-06-04
**Status:** Approved

## Goal

A dev-only page at `/dev/colors` that renders all global CSS color tokens from `globals.css` in a readable, grouped layout with a light/dark theme toggle.

## Route

`/dev/colors` — `src/app/dev/colors/page.tsx`

- No layout wrapper (outside all route groups)
- No auth required
- Accessible in dev and production (read-only, no sensitive data)

## Page Structure

### Header
- Title: "Color Tokens"
- Theme toggle button (☀ / 🌙) top-right — adds/removes `.dark` class on `<html>`

### Token Card
Each token renders as:
```
[████████] --token-name
            #RRGGBB   ← resolved via getComputedStyle at runtime
```
- Color swatch: 48×48px rectangle, background = `var(--token-name)`
- Token name in monospace
- Hex value resolved client-side via `getComputedStyle(document.documentElement).getPropertyValue('--token-name')`

### Sections (in order)

1. **Base** — `background`, `foreground`, `card`, `card-foreground`, `popover`, `popover-foreground`, `primary`, `primary-light`, `primary-foreground`, `secondary`, `secondary-foreground`, `muted`, `muted-foreground`, `accent`, `accent-foreground`, `destructive`, `border`, `input`, `ring`

2. **Semantic** — `success`, `warning`, `purple`, `danger`

3. **Sidebar** — `sidebar`, `sidebar-foreground`, `sidebar-primary`, `sidebar-primary-foreground`, `sidebar-accent`, `sidebar-accent-foreground`, `sidebar-border`, `sidebar-ring`, `sidebar-active-bg`, `sidebar-active-text`, `sidebar-hover-bg`, `sidebar-text`

4. **Status chips** — For each status (`scheduled`, `completed`, `cancelled`, `missed`): show bg + text pair as a live `<StatusBadge>`-style chip alongside two swatches

5. **Calendar chips** — For each status: three swatches (bg, border, text) labelled accordingly

6. **Charts** — `chart-1` through `chart-5`

## Implementation Notes

- Client component (`"use client"`) to resolve `getComputedStyle` values
- Theme toggle writes to `localStorage` key `theme` and syncs with `document.documentElement.classList`
- No external dependencies beyond what already exists in the project
- Tokens defined in `globals.css` `:root` / `.dark` — no duplication needed

## Files

| File | Action |
|---|---|
| `src/app/dev/colors/page.tsx` | Create |
