# Миграция доски: tldraw → Excalidraw

**Дата:** 2026-07-10
**Статус:** утверждён

## Зачем

tldraw 5.x блокирует редактор на проде без лицензионного ключа; текущий пробный
ключ истекает ~2026-10-06, платная лицензия дорогая. Excalidraw — MIT, без
ключей и водяных знаков. Миграция снимает дедлайн навсегда.

Следующая фича (библиотека учебников с вставкой страниц на доску) будет
строиться уже поверх Excalidraw — отдельный spec.

## Решения, принятые с пользователем

1. **Старый контент досок обнуляется.** Продукт до запуска; конвертер
   tldraw→Excalidraw не пишем. Фронт при загрузке распознаёт формат снапшота
   (нет поля `elements` → старый tldraw) и стартует с чистой доски; первый
   дебаунс-снапшот перезаписывает данные в новом формате. Ни SQL-миграции, ни
   скрипта очистки.
2. **UI — дефолтный Excalidraw** (`langCode="ru"`). Кастомный paper-UI
   (`BoardUi.tsx`, `boardTools.ts`, поповер настроек) удаляется. Своим остаётся
   только меню страниц, бейдж реконнекта и гостевой режим.
3. **Порядок работ:** сначала эта миграция, потом библиотека учебников.

## Архитектура

### Зависимости

- Удалить: `@tldraw/tldraw`, env `NEXT_PUBLIC_TLDRAW_LICENSE_KEY` (код и деплой).
- Добавить: `@excalidraw/excalidraw@^0.18.1` (peer deps: React 17/18/19 — наш
  React 19.2 поддержан).
- Компонент канваса грузится через `next/dynamic` с `ssr: false` — Excalidraw
  не рендерится на сервере.

### Бэкенд: без изменений

Go-хаб (`handlers/whiteboard_ws.go`) гоняет `snapshot`/`update`/`cursor` как
опаковый `json.RawMessage`, ничего не парся внутри. Авторизация WS (JWT /
invite UUID), subscription-гейт, REST-эндпоинты страниц и ассетов — не
трогаются. Новый тип сообщения `file` попадает в ветку `default` switch'а и
ретранслируется verbatim — это уже работает.

Ограничение, которое формирует дизайн: `SetReadLimit(512 * 1024)` — ни одно
WS-сообщение не может превышать 512 КБ. Отсюда запрет на base64 в снапшотах.

### Синхронизация: `useExcalidrawSync` (замена `useWhiteboardSync`)

Протокол WS сохраняется: `snapshot` (персистится хабом), `update` (ретранслируется,
не персистится), `cursor` (ретранслируется с peerId), новый `file`
(ретранслируется).

**Исходящее:**
- `onChange(elements)` → дифф по карте `id → version` (локальный `Map`,
  обновляется на каждом вызове) → изменённые/новые элементы шлются как
  `update { elements: [...] }` с throttle ~100 мс. Удаления не требуют
  отдельной обработки: Excalidraw помечает элементы `isDeleted: true` и
  bump'ает `version`, так что tombstone едет как обычный update.
- Дебаунс 1 с → `snapshot { elements, files }`, где `files` — карта-указатель
  `{ fileId: { url, mimeType } }` без данных (см. «Картинки»).

**Входящее:**
- `snapshot` (при коннекте) → сидим `elements` через
  `excalidrawAPI.updateScene()`, затем гидрируем файлы (fetch по URL →
  dataURL → `addFiles`).
- `update` → `reconcileElements(localElements, remoteElements, appState)` →
  `updateScene()`. `reconcileElements` — публичный экспорт
  `@excalidraw/excalidraw`, сливает по `version`/`versionNonce`.
- `file` → fetch URL → dataURL → `addFiles`.
- Эхо-петля: элементы, пришедшие из `updateScene()`, снова попадут в
  `onChange`; карта версий уже содержит их версии, дифф пуст — повторной
  отправки нет.

**Реконнект:** текущая выстраданная логика переносится как есть — `closedRef`
против зомби-reconnect'ов, `wsRef.current !== ws`-гард в `onclose`, retry 2 с,
`getTokenAsync` перед коннектом, финальный снапшот в cleanup.

### Картинки

Модель Excalidraw: image-элемент несёт `fileId`; данные живут в отдельной
карте `files` как base64 `dataURL`. Base64 в снапшоты класть нельзя (512 КБ
ReadLimit + распухание Postgres). Схема:

- **Вставка:** blob → S3 через существующий `whiteboardApi.uploadAsset` →
  локально `addFiles([{ id, dataURL }])` из blob'а (без повторного скачивания)
  → WS `file { fileId, url, mimeType }` → создание image-элемента.
- **Приёмник / загрузка страницы:** fetch по URL (presigned-redirect эндпоинт,
  как сейчас) → blob → dataURL → `addFiles`.
- **Персист:** в снапшоте — только `files: { fileId: { url, mimeType } }`.
  S3 остаётся источником истины, как в текущей tldraw-схеме.

### PDF-поток

`lib/pdf.ts`, `lib/pdfRange.ts`, `PdfRangeDialog.tsx` — без изменений
(движок-агностичны). Меняется только клей в канвас-компоненте: вместо
`editor.createAssets`/`createShape` — схема «Картинки» выше, по одному
image-элементу на страницу, встык по горизонтали (как сейчас).

Drop PDF на канвас: у Excalidraw нет аналога `experimental__onDropOnCanvas` —
перехватываем `onDropCapture` на div-обёртке; PDF → `preventDefault` +
диалог диапазона, остальное пропускаем в Excalidraw (нативная вставка
картинок drag-drop'ом у него есть, но она кладёт base64 — для v1 приемлемо
для мелких картинок; вставка через нашу кнопку идёт через S3).

### Курсоры

Самодельный оверлей заменяется нативными `collaborators` Excalidraw:
- `onPointerUpdate` → WS `cursor { x, y }` (throttle как сейчас).
- Входящий `cursor` → `updateScene({ collaborators: Map })` — именованные
  курсоры рисует сам Excalidraw. Протокол WS не меняется.

### UI

- Дефолтный UI Excalidraw, `langCode="ru"`.
- `BoardPageMenu` монтируется через проп `renderTopRightUI`.
- Бейдж «Переподключение...» — поверх, как сейчас.
- Гость: паритет с текущим поведением — гость рисует, скрыта только вставка
  картинок (наша кнопка не рендерится при `isGuest`). `viewModeEnabled` не
  используем.

### Удаляется

- `frontend/src/components/whiteboard/BoardUi.tsx`
- `frontend/src/components/whiteboard/boardTools.ts` + `boardTools.test.ts`
- `TldrawCanvas.tsx` → заменяется `ExcalidrawCanvas.tsx`
- `useWhiteboardSync.ts` → заменяется `useExcalidrawSync.ts`
- `HIDDEN_UI`, `licenseKey`, все импорты `@tldraw/*`

### Остаётся без изменений

- `BoardContext.tsx`, `BoardPageMenu.tsx`, `InviteSharePanel.tsx`,
  `PdfRangeDialog.tsx`
- `lib/pdf.ts`, `lib/pdfRange.ts`, `lib/api/whiteboard.ts`
- Весь Go-бэкенд

## Тестирование

- node:test на чистую функцию диффа версий (`diffChangedElements(prevVersions,
  elements)`) — единственная нетривиальная логика, выносимая из React.
- Ручная проверка: два окна — рисование синхронизируется в обе стороны;
  reload — контент восстановился; вставка картинки/PDF — видна во втором
  окне и после reload; гость по invite-ссылке рисует; старый tldraw-снапшот →
  чистая доска без ошибок.

## Риски

- Тач/стилус в Excalidraw ведёт себя иначе, чем в tldraw — проверить на
  планшете после мержа.
- `@excalidraw/excalidraw` — версия 0.x: минорные апгрейды могут ломать API
  (`reconcileElements` публичен, но семвер-гарантий нет). Пиновать `^0.18.1`.
- Нативный drag-drop картинок Excalidraw кладёт base64 в `files` — большая
  брошенная картинка может раздуть снапшот. Митигция v1: перехватывать в
  `onChange` файлы без записи в нашей `files`-карте URL и догружать их в S3
  фоном — **отложено**, в v1 принимаем риск (кнопка вставки идёт через S3,
  drag-drop мелких картинок редок). Отметить ponytail-комментарием.
