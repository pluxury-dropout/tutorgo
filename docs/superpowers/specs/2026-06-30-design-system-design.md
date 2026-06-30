# Design System — tutorgo

**Цель:** загрузить дизайн-систему tutorgo в claude.ai через DesignSync, чтобы Claude понимал кастомные компоненты и токены при генерации кода.

## Что загружаем

Только то, что уникально для tutorgo. Стандартные shadcn/ui компоненты (Button, Input, Dialog…) пропускаем — Claude их знает без контекста.

## Файлы

Папка: `frontend/ds/` — 6 самодостаточных HTML-файлов.

Каждый файл:
- Первая строка: `<!-- @dsCard group="..." -->` (claude.ai использует это для индексации)
- Инлайн-CSS с реальными hex-значениями из `:root` в `globals.css`
- Два блока: светлая тема (#F8F8F9 фон) | тёмная тема (#191919 фон) — side-by-side

| Файл | Группа | Содержимое |
|------|--------|-----------|
| `tokens.html` | Foundations | Цвета (primary, muted, destructive, success, warning), статусные цвета (4 статуса × light/dark), радиусы, тени |
| `status-badge.html` | Badges | 6 вариантов: scheduled / completed / cancelled / missed / active / ended |
| `course-type-badge.html` | Badges | Индивидуальный (primary-light bg) + групповой (muted bg) |
| `cycle-badge.html` | Badges | Позиция в цикле: обычная (серый, opacity 0.6) + последняя в цикле (красный круг) |
| `page-header.html` | Layout | PageHeader с title + meta + actions; HeaderMetric с цветной точкой |
| `empty-state.html` | Feedback | EmptyState: иконка в круге primary/8, заголовок, описание, кнопка |

## Токены (значения из `:root`)

```
--background:       #F8F8F9   dark: #191919
--foreground:       #1B1C1F   dark: #E3E2E0
--primary:          #222428   dark: #7e7e7e
--primary-light:    #ECECEE   dark: #3e3e3e
--muted:            #EFEFF1   dark: #2a2a2a
--muted-foreground: #646670   dark: #979A9B
--border:           #E1E1E4   dark: #2e2e2e
--destructive:      #C92A2A   dark: #CD4945
--success:          #077A4E   dark: #2D9964
--warning:          #B45309   dark: #CA8E1B
--radius:           0.5rem
--shadow-card:      0 1px 3px rgba(0,0,0,0.06)
```

## После создания файлов

1. DesignSync: `list_projects` — проверить есть ли уже design-system проект
2. Если нет — `create_project` с именем "tutorgo"
3. `finalize_plan` с путями всех 6 файлов из `frontend/ds/`
4. `write_files` — загрузить все файлы

## Что НЕ входит

- shadcn/ui компоненты (Button, Input, Card, Dialog, Select, Tabs…) — Claude знает их
- FullCalendar стили — слишком специфичны для библиотеки, не кастомные компоненты
- KanbanWidget, CallRoom, TldrawCanvas — составные фичи, не атомарные компоненты
