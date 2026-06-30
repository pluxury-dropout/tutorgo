# Design System — DesignSync Upload Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Создать 6 самодостаточных HTML-превью компонентов tutorgo и загрузить их в claude.ai через DesignSync.

**Architecture:** Папка `frontend/ds/` содержит изолированные HTML-файлы с инлайн-CSS и реальными hex-значениями токенов. Каждый файл — light + dark side-by-side. После создания всех файлов — один вызов DesignSync для загрузки.

**Tech Stack:** HTML + inline CSS, DesignSync tool (claude.ai)

## Global Constraints

- Первая строка каждого HTML-файла: `<!-- @dsCard group="..." -->` (без пробела перед `<!--`)
- Все цвета — hardcoded hex из `globals.css :root` и `.dark`, никаких CSS переменных (standalone HTML не имеет контекста)
- Никаких внешних зависимостей в HTML-файлах
- Файлы кладём в `frontend/ds/`

---

## File Map

| Файл | Группа |
|------|--------|
| `frontend/ds/tokens.html` | Foundations |
| `frontend/ds/status-badge.html` | Badges |
| `frontend/ds/course-type-badge.html` | Badges |
| `frontend/ds/cycle-badge.html` | Badges |
| `frontend/ds/page-header.html` | Layout |
| `frontend/ds/empty-state.html` | Feedback |

---

## Task 1: Tokens preview

**Files:**
- Create: `frontend/ds/tokens.html`

- [ ] **Step 1: Создать frontend/ds/tokens.html**

```html
<!-- @dsCard group="Foundations" -->
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
body{margin:0;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;font-size:13px;display:flex;}
.t{flex:1;padding:20px;}
.l{background:#F8F8F9;color:#1B1C1F;}
.d{background:#191919;color:#E3E2E0;}
.lbl{font-size:10px;font-weight:600;opacity:.4;text-transform:uppercase;letter-spacing:.06em;margin-bottom:14px;}
.section{margin-bottom:16px;}
.section-title{font-size:10px;font-weight:600;opacity:.5;margin-bottom:6px;}
.swatches{display:flex;flex-wrap:wrap;gap:6px;}
.swatch{display:flex;flex-direction:column;align-items:center;gap:3px;}
.sw{width:36px;height:28px;border-radius:5px;}
.sw-name{font-size:9px;opacity:.6;}
.chips{display:flex;flex-wrap:wrap;gap:5px;}
.radii{display:flex;gap:8px;align-items:flex-end;}
.r{background:currentColor;opacity:.15;height:28px;}
</style>
</head>
<body>
<div class="t l">
  <div class="lbl">Light</div>
  <div class="section">
    <div class="section-title">Цвета</div>
    <div class="swatches">
      <div class="swatch"><div class="sw" style="background:#222428;border:1px solid #E1E1E4"></div><span class="sw-name">primary</span></div>
      <div class="swatch"><div class="sw" style="background:#ECECEE;border:1px solid #E1E1E4"></div><span class="sw-name">primary-light</span></div>
      <div class="swatch"><div class="sw" style="background:#EFEFF1;border:1px solid #E1E1E4"></div><span class="sw-name">muted</span></div>
      <div class="swatch"><div class="sw" style="background:#646670;border:1px solid #E1E1E4"></div><span class="sw-name">muted-fg</span></div>
      <div class="swatch"><div class="sw" style="background:#F8F8F9;border:1px solid #E1E1E4"></div><span class="sw-name">background</span></div>
      <div class="swatch"><div class="sw" style="background:#E1E1E4;border:1px solid #E1E1E4"></div><span class="sw-name">border</span></div>
      <div class="swatch"><div class="sw" style="background:#C92A2A"></div><span class="sw-name">destructive</span></div>
      <div class="swatch"><div class="sw" style="background:#077A4E"></div><span class="sw-name">success</span></div>
      <div class="swatch"><div class="sw" style="background:#B45309"></div><span class="sw-name">warning</span></div>
    </div>
  </div>
  <div class="section">
    <div class="section-title">Статусы</div>
    <div class="chips">
      <span style="padding:3px 9px;border-radius:20px;font-size:11px;font-weight:600;background:#DCE8FE;color:#1D4ED8">scheduled</span>
      <span style="padding:3px 9px;border-radius:20px;font-size:11px;font-weight:600;background:#D2F2E2;color:#077A4E">completed</span>
      <span style="padding:3px 9px;border-radius:20px;font-size:11px;font-weight:600;background:#EFEFF1;color:#646670">cancelled</span>
      <span style="padding:3px 9px;border-radius:20px;font-size:11px;font-weight:600;background:#FCE2E2;color:#C92A2A">missed</span>
    </div>
  </div>
  <div class="section">
    <div class="section-title">Радиусы</div>
    <div class="radii">
      <div style="display:flex;flex-direction:column;align-items:center;gap:3px;">
        <div style="width:28px;height:20px;background:#222428;opacity:.15;border-radius:3px;"></div><span style="font-size:9px;opacity:.6">sm·0.3rem</span>
      </div>
      <div style="display:flex;flex-direction:column;align-items:center;gap:3px;">
        <div style="width:28px;height:20px;background:#222428;opacity:.15;border-radius:4px;"></div><span style="font-size:9px;opacity:.6">md·0.4rem</span>
      </div>
      <div style="display:flex;flex-direction:column;align-items:center;gap:3px;">
        <div style="width:28px;height:20px;background:#222428;opacity:.15;border-radius:8px;"></div><span style="font-size:9px;opacity:.6">default·0.5rem</span>
      </div>
      <div style="display:flex;flex-direction:column;align-items:center;gap:3px;">
        <div style="width:28px;height:20px;background:#222428;opacity:.15;border-radius:12px;"></div><span style="font-size:9px;opacity:.6">lg·0.75rem</span>
      </div>
    </div>
  </div>
  <div class="section">
    <div class="section-title">Тень карточки</div>
    <div style="display:inline-block;padding:8px 14px;background:#FFFFFF;border-radius:8px;box-shadow:0 1px 3px rgba(0,0,0,0.06);font-size:12px;">shadow-card</div>
  </div>
</div>
<div class="t d">
  <div class="lbl">Dark</div>
  <div class="section">
    <div class="section-title" style="color:#E3E2E0">Цвета</div>
    <div class="swatches">
      <div class="swatch"><div class="sw" style="background:#7e7e7e;border:1px solid #2e2e2e"></div><span class="sw-name" style="color:#979A9B">primary</span></div>
      <div class="swatch"><div class="sw" style="background:#3e3e3e;border:1px solid #2e2e2e"></div><span class="sw-name" style="color:#979A9B">primary-light</span></div>
      <div class="swatch"><div class="sw" style="background:#2a2a2a;border:1px solid #2e2e2e"></div><span class="sw-name" style="color:#979A9B">muted</span></div>
      <div class="swatch"><div class="sw" style="background:#979A9B;border:1px solid #2e2e2e"></div><span class="sw-name" style="color:#979A9B">muted-fg</span></div>
      <div class="swatch"><div class="sw" style="background:#222222;border:1px solid #2e2e2e"></div><span class="sw-name" style="color:#979A9B">card</span></div>
      <div class="swatch"><div class="sw" style="background:#2e2e2e;border:1px solid #3e3e3e"></div><span class="sw-name" style="color:#979A9B">border</span></div>
      <div class="swatch"><div class="sw" style="background:#CD4945"></div><span class="sw-name" style="color:#979A9B">destructive</span></div>
      <div class="swatch"><div class="sw" style="background:#2D9964"></div><span class="sw-name" style="color:#979A9B">success</span></div>
      <div class="swatch"><div class="sw" style="background:#CA8E1B"></div><span class="sw-name" style="color:#979A9B">warning</span></div>
    </div>
  </div>
  <div class="section">
    <div class="section-title" style="color:#E3E2E0">Статусы</div>
    <div class="chips">
      <span style="padding:3px 9px;border-radius:20px;font-size:11px;font-weight:600;background:#1F282D;color:#447ACB">scheduled</span>
      <span style="padding:3px 9px;border-radius:20px;font-size:11px;font-weight:600;background:#242B26;color:#4F9768">completed</span>
      <span style="padding:3px 9px;border-radius:20px;font-size:11px;font-weight:600;background:#252525;color:#9B9B9B">cancelled</span>
      <span style="padding:3px 9px;border-radius:20px;font-size:11px;font-weight:600;background:#332523;color:#BE524B">missed</span>
    </div>
  </div>
  <div class="section">
    <div class="section-title" style="color:#E3E2E0">Радиусы</div>
    <div class="radii">
      <div style="display:flex;flex-direction:column;align-items:center;gap:3px;">
        <div style="width:28px;height:20px;background:#E3E2E0;opacity:.15;border-radius:3px;"></div><span style="font-size:9px;color:#979A9B">sm</span>
      </div>
      <div style="display:flex;flex-direction:column;align-items:center;gap:3px;">
        <div style="width:28px;height:20px;background:#E3E2E0;opacity:.15;border-radius:4px;"></div><span style="font-size:9px;color:#979A9B">md</span>
      </div>
      <div style="display:flex;flex-direction:column;align-items:center;gap:3px;">
        <div style="width:28px;height:20px;background:#E3E2E0;opacity:.15;border-radius:8px;"></div><span style="font-size:9px;color:#979A9B">default</span>
      </div>
      <div style="display:flex;flex-direction:column;align-items:center;gap:3px;">
        <div style="width:28px;height:20px;background:#E3E2E0;opacity:.15;border-radius:12px;"></div><span style="font-size:9px;color:#979A9B">lg</span>
      </div>
    </div>
  </div>
  <div class="section">
    <div class="section-title" style="color:#E3E2E0">Тень карточки</div>
    <div style="display:inline-block;padding:8px 14px;background:#222222;border-radius:8px;box-shadow:0 1px 3px rgba(0,0,0,0.3);font-size:12px;color:#E3E2E0;">shadow-card</div>
  </div>
</div>
</body>
</html>
```

- [ ] **Step 2: Проверить файл в браузере**

```bash
open frontend/ds/tokens.html
# или на Linux:
xdg-open frontend/ds/tokens.html
```

Ожидание: два столбца (light/dark), свотчи цветов, статусные чипы, радиусы, тень.

- [ ] **Step 3: Commit**

```bash
git add frontend/ds/tokens.html
git commit -m "feat(ds): add tokens preview for DesignSync"
```

---

## Task 2: StatusBadge preview

**Files:**
- Create: `frontend/ds/status-badge.html`

- [ ] **Step 1: Создать frontend/ds/status-badge.html**

```html
<!-- @dsCard group="Badges" -->
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
body{margin:0;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;font-size:13px;display:flex;}
.t{flex:1;padding:20px;}
.l{background:#F8F8F9;color:#1B1C1F;}
.d{background:#191919;color:#E3E2E0;}
.lbl{font-size:10px;font-weight:600;opacity:.4;text-transform:uppercase;letter-spacing:.06em;margin-bottom:14px;}
.row{display:flex;flex-wrap:wrap;gap:6px;margin-bottom:10px;}
.badge{display:inline-flex;align-items:center;border-radius:20px;font-weight:600;padding:3px 9px;font-size:12px;}
.badge-sm{font-size:11px;padding:1px 8px;}
.section-title{font-size:10px;font-weight:600;opacity:.5;margin-bottom:6px;}
</style>
</head>
<body>
<div class="t l">
  <div class="lbl">Light</div>
  <div class="section-title">size=md (default)</div>
  <div class="row">
    <span class="badge" style="background:#DCE8FE;color:#1D4ED8">Запланирован</span>
    <span class="badge" style="background:#D2F2E2;color:#077A4E">Завершён</span>
    <span class="badge" style="background:#EFEFF1;color:#646670">Отменён</span>
    <span class="badge" style="background:#FCE2E2;color:#C92A2A">Пропущен</span>
    <span class="badge" style="background:#D2F2E2;color:#077A4E">Активный</span>
    <span class="badge" style="background:#EFEFF1;color:#646670">Завершён</span>
  </div>
  <div class="section-title">size=sm</div>
  <div class="row">
    <span class="badge badge-sm" style="background:#DCE8FE;color:#1D4ED8">Запланирован</span>
    <span class="badge badge-sm" style="background:#D2F2E2;color:#077A4E">Завершён</span>
    <span class="badge badge-sm" style="background:#EFEFF1;color:#646670">Отменён</span>
    <span class="badge badge-sm" style="background:#FCE2E2;color:#C92A2A">Пропущен</span>
  </div>
</div>
<div class="t d">
  <div class="lbl">Dark</div>
  <div class="section-title" style="color:#E3E2E0">size=md (default)</div>
  <div class="row">
    <span class="badge" style="background:#1F282D;color:#447ACB">Запланирован</span>
    <span class="badge" style="background:#242B26;color:#4F9768">Завершён</span>
    <span class="badge" style="background:#252525;color:#9B9B9B">Отменён</span>
    <span class="badge" style="background:#332523;color:#BE524B">Пропущен</span>
    <span class="badge" style="background:#242B26;color:#4F9768">Активный</span>
    <span class="badge" style="background:#252525;color:#9B9B9B">Завершён</span>
  </div>
  <div class="section-title" style="color:#E3E2E0">size=sm</div>
  <div class="row">
    <span class="badge badge-sm" style="background:#1F282D;color:#447ACB">Запланирован</span>
    <span class="badge badge-sm" style="background:#242B26;color:#4F9768">Завершён</span>
    <span class="badge badge-sm" style="background:#252525;color:#9B9B9B">Отменён</span>
    <span class="badge badge-sm" style="background:#332523;color:#BE524B">Пропущен</span>
  </div>
</div>
</body>
</html>
```

- [ ] **Step 2: Проверить в браузере**

```bash
open frontend/ds/status-badge.html
```

Ожидание: 6 чипов в md, 4 в sm; light left / dark right.

- [ ] **Step 3: Commit**

```bash
git add frontend/ds/status-badge.html
git commit -m "feat(ds): add StatusBadge preview for DesignSync"
```

---

## Task 3: CourseTypeBadge + CycleBadge previews

**Files:**
- Create: `frontend/ds/course-type-badge.html`
- Create: `frontend/ds/cycle-badge.html`

- [ ] **Step 1: Создать frontend/ds/course-type-badge.html**

```html
<!-- @dsCard group="Badges" -->
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
body{margin:0;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;font-size:13px;display:flex;}
.t{flex:1;padding:20px;}
.l{background:#F8F8F9;color:#1B1C1F;}
.d{background:#191919;color:#E3E2E0;}
.lbl{font-size:10px;font-weight:600;opacity:.4;text-transform:uppercase;letter-spacing:.06em;margin-bottom:14px;}
.badge{display:inline-flex;align-items:center;gap:4px;padding:2px 8px;border-radius:9999px;font-size:12px;font-weight:500;}
.row{display:flex;gap:8px;flex-wrap:wrap;}
</style>
</head>
<body>
<div class="t l">
  <div class="lbl">Light</div>
  <div class="row">
    <span class="badge" style="background:#ECECEE;color:#222428">
      <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>
      Индивидуальный
    </span>
    <span class="badge" style="background:#EFEFF1;color:#646670">
      <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
      Групповой
    </span>
  </div>
</div>
<div class="t d">
  <div class="lbl">Dark</div>
  <div class="row">
    <span class="badge" style="background:#3e3e3e;color:#c4c4c4">
      <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>
      Индивидуальный
    </span>
    <span class="badge" style="background:#2a2a2a;color:#979A9B">
      <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
      Групповой
    </span>
  </div>
</div>
</body>
</html>
```

- [ ] **Step 2: Создать frontend/ds/cycle-badge.html**

```html
<!-- @dsCard group="Badges" -->
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
body{margin:0;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;font-size:13px;display:flex;}
.t{flex:1;padding:20px;}
.l{background:#F8F8F9;color:#1B1C1F;}
.d{background:#191919;color:#E3E2E0;}
.lbl{font-size:10px;font-weight:600;opacity:.4;text-transform:uppercase;letter-spacing:.06em;margin-bottom:14px;}
.row{display:flex;align-items:center;gap:16px;margin-bottom:12px;}
.desc{font-size:11px;opacity:.5;margin-left:4px;}
</style>
</head>
<body>
<div class="t l">
  <div class="lbl">Light</div>
  <div class="row">
    <span style="font-size:10px;font-weight:600;opacity:.6;line-height:1;">2</span>
    <span class="desc">position=2, size=4 (обычная)</span>
  </div>
  <div class="row">
    <span style="display:inline-flex;align-items:center;justify-content:center;width:16px;height:16px;border-radius:50%;background:#b91c1c;color:#fff;font-size:10px;font-weight:700;line-height:1;">4</span>
    <span class="desc">position=4, size=4 (последняя в цикле)</span>
  </div>
  <div class="row" style="margin-top:8px;gap:6px;align-items:center;">
    <span style="font-size:10px;font-weight:600;opacity:.6;line-height:1;">1</span>
    <span style="font-size:10px;font-weight:600;opacity:.6;line-height:1;">2</span>
    <span style="font-size:10px;font-weight:600;opacity:.6;line-height:1;">3</span>
    <span style="display:inline-flex;align-items:center;justify-content:center;width:16px;height:16px;border-radius:50%;background:#b91c1c;color:#fff;font-size:10px;font-weight:700;line-height:1;">4</span>
    <span class="desc">цикл 4/4 в таблице уроков</span>
  </div>
</div>
<div class="t d">
  <div class="lbl">Dark</div>
  <div class="row">
    <span style="font-size:10px;font-weight:600;opacity:.6;line-height:1;color:#E3E2E0;">2</span>
    <span class="desc" style="color:#E3E2E0;">position=2, size=4 (обычная)</span>
  </div>
  <div class="row">
    <span style="display:inline-flex;align-items:center;justify-content:center;width:16px;height:16px;border-radius:50%;background:#b91c1c;color:#fff;font-size:10px;font-weight:700;line-height:1;">4</span>
    <span class="desc" style="color:#E3E2E0;">position=4, size=4 (последняя в цикле)</span>
  </div>
  <div class="row" style="margin-top:8px;gap:6px;align-items:center;">
    <span style="font-size:10px;font-weight:600;opacity:.6;line-height:1;color:#E3E2E0;">1</span>
    <span style="font-size:10px;font-weight:600;opacity:.6;line-height:1;color:#E3E2E0;">2</span>
    <span style="font-size:10px;font-weight:600;opacity:.6;line-height:1;color:#E3E2E0;">3</span>
    <span style="display:inline-flex;align-items:center;justify-content:center;width:16px;height:16px;border-radius:50%;background:#b91c1c;color:#fff;font-size:10px;font-weight:700;line-height:1;">4</span>
    <span class="desc" style="color:#E3E2E0;">цикл 4/4</span>
  </div>
</div>
</body>
</html>
```

- [ ] **Step 3: Проверить оба файла в браузере**

```bash
open frontend/ds/course-type-badge.html
open frontend/ds/cycle-badge.html
```

- [ ] **Step 4: Commit**

```bash
git add frontend/ds/course-type-badge.html frontend/ds/cycle-badge.html
git commit -m "feat(ds): add CourseTypeBadge and CycleBadge previews"
```

---

## Task 4: PageHeader + EmptyState previews

**Files:**
- Create: `frontend/ds/page-header.html`
- Create: `frontend/ds/empty-state.html`

- [ ] **Step 1: Создать frontend/ds/page-header.html**

```html
<!-- @dsCard group="Layout" -->
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
body{margin:0;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;font-size:14px;display:flex;}
.t{flex:1;padding:20px;}
.l{background:#F8F8F9;color:#1B1C1F;}
.d{background:#191919;color:#E3E2E0;}
.lbl{font-size:10px;font-weight:600;opacity:.4;text-transform:uppercase;letter-spacing:.06em;margin-bottom:14px;}
.header{border-bottom:1px solid;padding-bottom:14px;margin-bottom:18px;}
.header-row{display:flex;align-items:baseline;justify-content:space-between;gap:24px;flex-wrap:wrap;}
.header-left{display:flex;align-items:baseline;gap:10px;}
.h1{margin:0;font-size:17px;font-weight:600;letter-spacing:-.01em;}
.meta{font-size:13px;}
.btn{display:inline-flex;align-items:center;gap:6px;padding:5px 12px;border-radius:8px;font-size:13px;font-weight:500;border:none;cursor:pointer;}
.dot{display:inline-block;width:6px;height:6px;border-radius:50%;flex-shrink:0;}
.metric{display:inline-flex;align-items:baseline;gap:7px;}
.section-title{font-size:10px;font-weight:600;opacity:.5;margin-bottom:8px;}
</style>
</head>
<body>
<div class="t l">
  <div class="lbl">Light</div>
  <div class="section-title">С мета-текстом и кнопкой</div>
  <div class="header" style="border-color:#E1E1E4;">
    <div class="header-row">
      <div class="header-left">
        <h1 class="h1" style="color:#1B1C1F;">Студенты</h1>
        <span class="meta" style="color:#646670;">24 ученика</span>
      </div>
      <div>
        <button class="btn" style="background:#222428;color:#FFFFFF;">+ Добавить</button>
      </div>
    </div>
  </div>
  <div class="section-title">С HeaderMetric (цветная точка)</div>
  <div class="header" style="border-color:#E1E1E4;">
    <div class="header-row">
      <div class="header-left">
        <h1 class="h1" style="color:#1B1C1F;">Физика · Иванов А.</h1>
        <span class="meta" style="color:#646670;">
          <span class="metric">
            <span class="dot" style="background:#077A4E;transform:translateY(-1px);"></span>
            12 уроков
          </span>
        </span>
      </div>
      <div style="display:flex;gap:8px;">
        <button class="btn" style="background:#ECECEE;color:#222428;">Редактировать</button>
      </div>
    </div>
  </div>
</div>
<div class="t d">
  <div class="lbl">Dark</div>
  <div class="section-title" style="color:#E3E2E0;">С мета-текстом и кнопкой</div>
  <div class="header" style="border-color:#2e2e2e;">
    <div class="header-row">
      <div class="header-left">
        <h1 class="h1" style="color:#E3E2E0;">Студенты</h1>
        <span class="meta" style="color:#979A9B;">24 ученика</span>
      </div>
      <div>
        <button class="btn" style="background:#7e7e7e;color:#f0efed;">+ Добавить</button>
      </div>
    </div>
  </div>
  <div class="section-title" style="color:#E3E2E0;">С HeaderMetric</div>
  <div class="header" style="border-color:#2e2e2e;">
    <div class="header-row">
      <div class="header-left">
        <h1 class="h1" style="color:#E3E2E0;">Физика · Иванов А.</h1>
        <span class="meta" style="color:#979A9B;">
          <span class="metric">
            <span class="dot" style="background:#2D9964;transform:translateY(-1px);"></span>
            12 уроков
          </span>
        </span>
      </div>
      <div>
        <button class="btn" style="background:#3e3e3e;color:#E3E2E0;">Редактировать</button>
      </div>
    </div>
  </div>
</div>
</body>
</html>
```

- [ ] **Step 2: Создать frontend/ds/empty-state.html**

```html
<!-- @dsCard group="Feedback" -->
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
body{margin:0;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;font-size:14px;display:flex;}
.t{flex:1;padding:20px;}
.l{background:#F8F8F9;color:#1B1C1F;}
.d{background:#191919;color:#E3E2E0;}
.lbl{font-size:10px;font-weight:600;opacity:.4;text-transform:uppercase;letter-spacing:.06em;margin-bottom:14px;}
.empty{display:flex;flex-direction:column;align-items:center;justify-content:center;padding:40px 20px;text-align:center;}
.icon-wrap{width:64px;height:64px;border-radius:50%;display:flex;align-items:center;justify-content:center;margin-bottom:16px;}
.title{font-size:14px;font-weight:500;margin:0 0 4px;}
.desc{font-size:12px;margin:0 0 16px;}
.btn{padding:5px 12px;border-radius:8px;font-size:13px;font-weight:500;border:none;cursor:pointer;}
</style>
</head>
<body>
<div class="t l">
  <div class="lbl">Light</div>
  <div class="empty">
    <div class="icon-wrap" style="background:rgba(34,36,40,0.08);">
      <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="rgba(34,36,40,0.5)" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"/><path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"/></svg>
    </div>
    <p class="title" style="color:#1B1C1F;">Нет уроков</p>
    <p class="desc" style="color:#646670;">Добавьте первый урок для этого курса</p>
    <button class="btn" style="background:#222428;color:#FFFFFF;">Добавить урок</button>
  </div>
</div>
<div class="t d">
  <div class="lbl">Dark</div>
  <div class="empty">
    <div class="icon-wrap" style="background:rgba(126,126,126,0.08);">
      <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="rgba(126,126,126,0.5)" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"/><path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"/></svg>
    </div>
    <p class="title" style="color:#E3E2E0;">Нет уроков</p>
    <p class="desc" style="color:#979A9B;">Добавьте первый урок для этого курса</p>
    <button class="btn" style="background:#7e7e7e;color:#f0efed;">Добавить урок</button>
  </div>
</div>
</body>
</html>
```

- [ ] **Step 3: Проверить оба файла в браузере**

```bash
open frontend/ds/page-header.html
open frontend/ds/empty-state.html
```

- [ ] **Step 4: Commit**

```bash
git add frontend/ds/page-header.html frontend/ds/empty-state.html
git commit -m "feat(ds): add PageHeader and EmptyState previews"
```

---

## Task 5: Загрузить в claude.ai через DesignSync

**Requires:** Tasks 1–4 завершены, все 6 файлов в `frontend/ds/`

- [ ] **Step 1: Проверить существующие проекты**

Вызвать `DesignSync { method: "list_projects" }`.

- Если вернулся проект типа `design-system` — использовать его `projectId`.
- Если проектов нет — перейти к Step 2.
- Если есть другие проекты (не design-system) — игнорировать их.

- [ ] **Step 2: (если нужно) Создать проект**

Вызвать `DesignSync { method: "create_project", name: "tutorgo" }`.

Сохранить `projectId` из ответа.

- [ ] **Step 3: Узнать текущие файлы в проекте**

Вызвать `DesignSync { method: "list_files", projectId: "<id>" }`.

Если проект новый — список пустой.

- [ ] **Step 4: Зафиксировать план загрузки**

Вызвать `DesignSync { method: "finalize_plan", projectId: "<id>", localDir: "<абсолютный путь>/frontend/ds", writes: ["tokens.html", "status-badge.html", "course-type-badge.html", "cycle-badge.html", "page-header.html", "empty-state.html"] }`.

Сохранить `planId` из ответа.

- [ ] **Step 5: Загрузить файлы**

Вызвать `DesignSync { method: "write_files", planId: "<planId>", files: [
  { path: "tokens.html", localPath: "tokens.html" },
  { path: "status-badge.html", localPath: "status-badge.html" },
  { path: "course-type-badge.html", localPath: "course-type-badge.html" },
  { path: "cycle-badge.html", localPath: "cycle-badge.html" },
  { path: "page-header.html", localPath: "page-header.html" },
  { path: "empty-state.html", localPath: "empty-state.html" }
] }`.

- [ ] **Step 6: Проверить результат**

Открыть claude.ai → Projects → найти "tutorgo" в Design Systems. Должны быть видны 6 карточек в группах Foundations / Badges / Layout / Feedback.
