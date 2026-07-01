# Кастомный UI доски (tldraw) — дизайн

Дата: 2026-07-01

## Цель

Заменить дефолтный UI tldraw на кастомный по мокапу из handoff (`Board.dc.html`, тема
`paper`): верхний док-тулбар + поповер настроек. Плюс перевести PiP-видео звонка в
круглый вид сверху-справа.

Источник дизайна: `Дизайны интерактивной доски-handoff` → `Board.dc.html`, вариант
`variant="paper"`.

## Объём (решено с пользователем)

- Полностью свой UI: скрываем **весь** дефолтный UI tldraw (тулбар, стайл-панель,
  меню страниц, зум, контекстное меню). Хоткеи tldraw продолжают работать.
- Только тема **paper**. Константы захардкожены, без машинерии переключения тем (YAGNI).
- Видео: перестилизовать существующий `PipCameras` (реальные LiveKit-тайлы) под
  **круглый** PiP, **сверху-справа**, с живым видео и лейблом «Репетитор».

## Что из tldraw используем (проверено по докам и node_modules v5.1.0)

- Гасим видимый UI через `components={{ Toolbar: null, StylePanel: null, ContextMenu: null, ... }}`
  (пример `ui-components-hidden`), а НЕ через `hideUi`: так UI-обёртка tldraw продолжает
  монтироваться и хоткеи (Ctrl+Z, Delete, клавиши инструментов) остаются рабочими.
- Свой UI — как child `<Tldraw>`; внутри контекст редактора → `useEditor()`, реактивность
  через `track()` / `useValue`.
- `editor.setCurrentTool(id)` для инструментов; `editor.setStyleForNextShapes(...)` и
  `editor.setStyleForSelectedShapes(...)` для стилей.
- Константы стилей: `DefaultColorStyle`, `DefaultSizeStyle`, `DefaultFontStyle` — все есть.
- НЕ используем: `overrides`/`actions` (action-overrides) и `onUiEvent` (ui-events) —
  они для интеграции со встроенным UI-слоём; мы дёргаем `editor` напрямую.
- Готовые примитивы (`TldrawUiButton`/`TldrawUiPopover`) не берём: несут дефолтные стили,
  а поповер настроек открывается по клику на инструмент в доке, а не по своему триггеру.

## Ключевая ловушка (заложить сразу)

Кастомный UI — слой поверх холста. Обёртка `BoardUi` = `pointer-events:none`; каждый
интерактивный узел (док, поповер, видео) = `pointer-events:auto`. Иначе холст «мёртвый».

## Компоненты

### `boardTools.ts` (чистый модуль, тестируемый)
Маппинги без React:
- `TOOL_IDS`: порядок инструментов `['select','hand','pen','eraser','text','shape','sticky','image']`.
- `toEditorTool(uiTool)`: pen→`draw`, shape→`geo`, sticky→`note`, остальные — как есть; image→`null` (особый поток).
- `COLOR_MAP`: hex мокапа → имя цвета tldraw. `#26262a→black, #e0564f→red, #e6a43c→orange,
  #4f9d6e→green, #4f7bd0→blue, #9168d6→violet`.
- `SIZE_MAP`: `S→s, M→m, L→l, XL→xl`.
- `FONT_MAP`: `hand→draw, sans→sans, serif→serif`.
- Хелперы видимости поповера: `showColor/showThickness/showFont(uiTool)`.

### `BoardUi.tsx` (док + поповер настроек)
- `track()`-компонент, читает активный тул из `editor.getCurrentToolId()`.
- Локальный state `openTool` (какой поповер раскрыт); повторный клик по тому же
  инструменту закрывает. Логика из мокапа (`choose`).
- Док, 3 группы (грид `1fr auto 1fr`):
  - Левая: ☰ и чип «Урок · Стр. N» → открывают меню страниц (`BoardPageMenu`); разделитель;
    undo/redo (`editor.undo()/redo()`, disabled по `getCanUndo()/getCanRedo()`).
  - Центр: 8 кнопок-инструментов. Активная подсвечена (фон `#26262a`, иконка белая).
    image → триггерит скрытый `<input type=file>` → вставка ассета (переиспользуем логику
    вставки картинок из PDF-пути в `TldrawCanvas`).
  - Правая: 🗑 удалить выделенное (`editor.deleteShapes(editor.getSelectedShapeIds())`,
    disabled если пусто); ⋮ — заглушка (`ponytail:` add when needed).
- Поповер настроек (абсолютно под центром дока, анимация `bd-drop`):
  - цвет (pen/text/shape/sticky) · толщина (pen/eraser/shape) · шрифт+размер (text).
  - Клик по цвету/толщине/шрифту → `setStyleForNextShapes` **и** `setStyleForSelectedShapes`.
  - Подсветка активного значения — по текущему стилю. Точный реактивный геттер
    (`getStyleForNextShapes` vs `getInstanceState().stylesForNextShape`) — **уточнить в коде**.
  - Закрытие: повторный клик по инструменту; плюс небольшой обработчик клика снаружи.

### Правки существующих файлов
- `TldrawCanvas.tsx`: добавить `hideUi`, рендерить `<BoardUi />` как child `<Tldraw>`;
  вынести логику вставки картинки в вызываемую из `BoardUi` функцию (через ref/контекст,
  как уже сделано для PDF). Тему tldraw подогнать под фон мокапа (`#f4f1ea` + точечная сетка).
- `PipCameras.tsx`: круглый вид (маска `border-radius:50%`), сверху-справа под доком
  (`top ~72px, right 16px`), живое видео в круге, лейбл «Репетитор». Коллизии нет:
  `BoardToggleButton` — снизу-слева (`top:850, left:12`).
- `BoardPageMenu.tsx`: переиспользуем как содержимое меню страниц (лёгкая перестилизация
  под paper при необходимости).

## Тестирование

Нетривиальная логика — маппинги. Один прогоняемый тест `boardTools.test.ts` на `node:test`
(как в проекте): проверяет `toEditorTool`, `COLOR_MAP`, `SIZE_MAP`, `FONT_MAP`, хелперы
видимости. Без фреймворков.

## Файлы

Новые: `boardTools.ts`, `boardTools.test.ts`, `BoardUi.tsx`.
Правки: `TldrawCanvas.tsx`, `PipCameras.tsx`, (при необходимости) `BoardPageMenu.tsx`.
