# Дизайн: редизайн окна звонка — компактный тулбар

**Дата:** 2026-07-01
**Источник:** handoff `design_handoff_call_window` (Call Window.dc.html + README) от Claude design.
**Статус:** утверждён (объём и два отступления согласованы с пользователем).

## 1. Цель

Три «плавающих» управляющих элемента звонка схлопнуть в один компактный
нижний icon-only тулбар и добавить недостающие действия. Заменяем:

- дефолтный LiveKit `<ControlBar>` (mic/cam/leave/share) внутри `VideoGrid`;
- `BoardToggleButton` (плавал слева-снизу, только tutor);
- кнопку «копировать ссылку» (плавала справа-снизу, в `call/page.tsx`).

Net-new к этому: чат, меню «Ещё» (выбор устройств), модалка подтверждения
выхода, тост копирования ссылки.

Затрагивает обе поверхности, использующие `CallRoom`: страницу репетитора
`app/(call)/lessons/[id]/call/page.tsx` (role=tutor) и гостевой вход в урок
`app/join/[lessonId]/page.tsx` (role=guest). Quick-room `join/room/[id]`
использует дефолтный LiveKit `VideoConference` и в объём **не** входит.

## 2. Объём (решения по фичам)

| Фича | Решение |
|---|---|
| Чат | Реализуем через LiveKit DataChannel (без бэкенда, эфемерные сообщения). |
| Тема | Следуем теме приложения (`next-themes`, `resolvedTheme`). Chrome переключается, сцена/доска — постоянны. |
| Меню «Ещё» | Только «Настройки» (выбор камеры/микрофона). Полноэкранный/Запись/Сообщить о проблеме — не делаем. |
| Демонстрация экрана | Да, через LiveKit `setScreenShareEnabled`. |
| Экран «Вы покинули звонок» + rejoin | Не делаем — demo-only, текущий `onDisconnected` уже роутит назад. |

## 3. Осознанные отступления от «pixel-perfect» (согласованы)

1. **Доска остаётся богатой.** Существующий `BoardUi` (страницы, полный набор
   инструментов, палитра/размеры/шрифты) сохраняется. 4-цветный мини-тулбар из
   мокапа не реализуем — мокап сам помечает доску как placeholder.
2. **PiP-кружки остаются справа-сверху** (недавно зашипленная итерация), а не
   справа-снизу, как в хендоффе.

## 4. Архитектура

Всё живёт внутри существующего `CallRoomInner` (он всегда смонтирован внутри
`<LiveKitRoom>` — критично для слушателя чата). Дерево зависимостей проекта
не меняется.

### Новые файлы

| Файл | Роль | Зависит от |
|---|---|---|
| `components/call/CallStage.tsx` | Раскладка сцены участников. | LiveKit `useTracks`, `CallTile`, `layoutForCount` |
| `components/call/CallTile.tsx` | Один tile (видео/аватар/плашка имени). | `VideoTrack`, `ParticipantPlaceholder`, `useTrackMutedIndicator` |
| `components/call/CallToolbar.tsx` | Нижний pill + dropdown «Ещё» + модалка выхода + тост. | `useLocalParticipant`, `next-themes`, `DeviceSettings` |
| `components/call/CallChat.tsx` | Правая docked-панель чата (презентационная). | пропсы `messages`/`send`/`onClose` |
| `components/call/useCallChat.ts` | Хук DataChannel-чата. | `useRoomContext`, `RoomEvent.DataReceived` |
| `components/call/DeviceSettings.tsx` | Контент «Ещё»: выбор устройств. | `useMediaDeviceSelect` |
| `components/call/callTheme.ts` | Токены тем (dark/light) из хендоффа + `layoutForCount`. | — |

### Удаляем

- `components/call/BoardToggleButton.tsx`.
- `components/call/VideoGrid.tsx` (поглощается `CallStage`; дефолтный
  `ControlBar` уходит).
- Плавающую кнопку копирования и её `copied`-стейт из `call/page.tsx`.

### Изменяем

- `components/call/CallRoom.tsx` — `CallRoomInner` поднимает UI-стейт, монтирует
  `useCallChat`, рендерит `CallStage`/`CallToolbar`/`CallChat`, принимает
  `inviteUrl?`.
- `app/(call)/lessons/[id]/call/page.tsx` — передаёт `inviteUrl` в `CallRoom`,
  убирает свою кнопку копирования.

## 5. Компоненты — контракты

### `callTheme.ts`
- `themeTokens(resolved: 'dark'|'light')` → объект цветов (panel, border, text,
  muted, hover, accent, accentBg, destructive, destructiveBg, success) —
  значения из раздела Design Tokens хендоффа.
- Константы сцены (theme-independent): `STAGE_BG='#101113'`, `TILE_BG='#232427'`,
  `AVATAR='#5b5d63'`, `GLYPH='#3f4046'`.
- `layoutForCount(n)` → `{ mode: 'single'|'grid', columns: number }`.
  Правило: `n<=1 → single`; `n===2 → grid, columns 2`; `n>=3 → grid, columns 3`.
  (Не хардкодим 3 колонки — 1:1 урок = 2 участника, это основной кейс.)

### `CallTile.tsx`
- Props: `trackRef`, `isLocal`, размер (`speaker`|`grid`).
- Рендер: если это `TrackReference` с видео → `<VideoTrack objectFit:cover>`;
  иначе серый круг-аватар с глифом (168px в speaker, 62px в grid).
- Плашка имени **сверху-слева** (не снизу — низ занят тулбаром): тёмный
  полупрозрачный pill + иконка mic-muted (только если участник замьючен) + имя.
- Демонстрация экрана: источник `Track.Source.ScreenShare` уже в `useTracks`
  сцены → шара приходит как обычный дополнительный tile.
  Upgrade-path (не сейчас): focused-screenshare (большой + filmstrip камер).

### `CallStage.tsx`
- `useTracks([Camera {withPlaceholder}, ScreenShare])`, дедуп по
  `identity-source` (как в `PipCameras`).
- `layoutForCount(tracks.length)` → центрированный `CallTile` или CSS-grid.
- В board-mode не рендерится (рендерится `TldrawCanvas`+`PipCameras`, как сейчас).

### `CallToolbar.tsx`
- Props: `role`, `inviteUrl?`, `boardActive`, `chatActive`, `chatUnread`,
  `onToggleBoard`, `onToggleChat`, `onLeave`.
- Медиа-состояние/тумблеры — внутри, из `useLocalParticipant()`
  (`isMicrophoneEnabled`, `isCameraEnabled`, `isScreenShareEnabled` +
  `localParticipant.setXEnabled`). LiveKit — единый источник правды, своих
  булен-стейтов для медиа не держим.
- Порядок кнопок: Ссылка(tutor) → Микрофон → Камера → Экран → Доска(tutor) →
  Чат → Ещё → Выход. Guest не видит «Ссылка» и «Доска».
- Состояния кнопок (theme-aware): default (прозрач/text), active (accentBg/accent
  для share/board/chat/more), copied (successBg/success + галочка ~2.2s),
  leave (всегда destructiveBg/destructive). Hover — `opacity:0.8`.
- Копирование: `navigator.clipboard.writeText(inviteUrl)` → `linkCopied=true`,
  тост сверху-центр, авто-сброс через ~2.2s (`setTimeout`, чистить в cleanup).
- «Ещё»: dropdown вверх с `DeviceSettings`; открытие доски/чата закрывает «Ещё».
- Выход: модалка «Покинуть звонок?» (скрим `rgba(0,0,0,0.5)`) → «Покинуть»
  вызывает `onLeave` (= `room.disconnect()` в родителе).

### `useCallChat.ts`
- Живёт в `CallRoomInner` (всегда смонтирован) → слушатель `DataReceived` не
  отваливается при закрытом чате, сообщения не теряются, стейт не сбрасывается.
- Отдаёт `{ messages, send(text), unread, markRead() }`.
- Сообщение: `{ type:'chat', from, text }`. `from` = `localParticipant.name ||
  identity`. `messages: { from, text, mine }[]` в стейте.
- `unread` растёт на входящих, пока `chatOpen === false`; `markRead()` при
  открытии.

### `CallChat.tsx`
- Панель 280px справа, slide-in. Header «Чат» + close. Список сообщений
  (свои — accentBg справа, чужие — hover-tint слева). Инпут + круглая accent
  кнопка отправки, Enter-to-send. Автоскролл вниз при новом сообщении.
- Когда чат открыт, сцена/доска/PiP ужимаются на 280px справа
  (`transition: right .25s`).

### `DeviceSettings.tsx`
- `useMediaDeviceSelect({ kind:'videoinput' })` и `'audioinput'` — списки
  устройств + активное, выбор переключает трек.

## 6. Потоки данных

- **Медиа:** UI ← LiveKit-хуки; действие → `setXEnabled`.
- **Доска:** без изменений. `mode` (call/board) и `handleToggle` уже в
  `CallRoomInner`. Кнопка «Доска» отражает `mode==='board'`; для guest скрыта
  (доска открывается по DataChannel от tutor).
- **Один DataChannel на доску и чат:** union
  `{type:'board-open'|'board-close'|'chat', ...}`. **Оба** обработчика
  (доска в `CallRoomInner`, чат в `useCallChat`) проверяют `type`, чтобы не
  глотать чужие сообщения.
- **Тема:** `resolvedTheme` → токены `callTheme`. Переключается только chrome.
- **Копирование ссылки:** `inviteUrl` приходит пропом со страницы
  (`${origin}/join/${lessonId}`); `CallRoom` сам `id` не знает.
- **Выход:** модалка → `onLeave` → `room.disconnect()` → существующий
  `onDisconnected` (endRoom + `router.back()`).

## 7. Роли

| | tutor | guest |
|---|---|---|
| Ссылка | ✅ | ❌ |
| Доска (кнопка) | ✅ | ❌ (открывается по DataChannel) |
| Микрофон/Камера/Экран/Чат/Ещё/Выход | ✅ | ✅ |

## 8. Ошибки

- Ошибки медиа/доски/копирования → `toast.error` (sonner, как в текущем коде).
- `clipboard.writeText` в try/catch; таймеры тоста/copied чистятся в cleanup.

## 9. Тесты

Проект использует `node:test` + Node TS-strip (нет vitest/RTL), поэтому
покрываем чистую логику, не UI:

- `layoutForCount(n)` — граничные `0,1,2,3,4` → ожидаемые mode/columns.
- сериализация/парсинг чат-сообщения + дискриминант `type` (чат не парсит
  board-сообщения и наоборот).

## 10. Вне объёма (YAGNI / upgrade-paths)

- Focused-screenshare раскладка (большой шар + filmstrip).
- Полноэкранный режим, запись звонка, «сообщить о проблеме».
- Экран «Вы покинули звонок» + rejoin.
- Перенос PiP вниз-вправо, 4-цветный мини-тулбар доски.
- Персистентность истории чата (сейчас эфемерная).
