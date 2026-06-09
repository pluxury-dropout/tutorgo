# Call Fullscreen Layout

**Date:** 2026-06-03  
**Status:** Approved

## Problem

`/lessons/[id]/call` и `/room/[id]` находятся в route group `(dashboard)`, из-за чего LiveKit VideoConference рендерится с сайдбаром и мобильной шапкой. Высота фиксируется хаком `calc(100vh - 64px)`.

## Solution

Вынести call/room страницы в отдельный route group `(call)` с минимальным лейаутом — только auth-проверка, без сайдбара.

## File Structure

```
app/
  (call)/
    layout.tsx                        # новый — auth + overflow-hidden, без сайдбара
    lessons/[id]/call/page.tsx        # перенести из (dashboard)
    room/[id]/page.tsx                # перенести из (dashboard)
  (dashboard)/
    lessons/[id]/call/                # удалить
    room/[id]/                        # удалить
```

## Layout `(call)/layout.tsx`

- `'use client'`
- Читает `token` из `useAuthStore`, на `!mounted || !isAuthenticated` редиректит на `/login` — идентично DashboardLayout
- Рендерит `<div className="h-screen overflow-hidden">{children}</div>`
- Нет Sidebar, MobileBottomNav, мобильной шапки

## Page Changes

В обоих page.tsx: `height: 'calc(100vh - 64px)'` → `height: '100dvh'`.  
Логика страниц не меняется.

## URLs

Не меняются: `/lessons/[id]/call` и `/room/[id]` работают как раньше.

## Out of Scope

- `join/*` страницы — уже полноэкранные, трогать не нужно
- Изменения логики LiveKit
