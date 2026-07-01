# Call Window Compact-Toolbar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Заменить плавающие управляющие элементы звонка (LiveKit ControlBar, кнопку доски, кнопку копирования ссылки) единым компактным нижним тулбаром + добавить чат, меню устройств, модалку выхода.

**Architecture:** Всё живёт внутри существующего `CallRoomInner` (всегда смонтирован в `<LiveKitRoom>`). Медиа-состояние читаем из LiveKit-хуков (единый источник правды). Сцена участников и чат — кастомные компоненты на LiveKit-примитивах. Тема chrome следует `next-themes`; сцена/доска — постоянного цвета. Чат и доска делят один DataChannel с дискриминантом `type`.

**Tech Stack:** Next.js 16 (App Router, client components), React, TypeScript, `@livekit/components-react` + `livekit-client`, `next-themes`, `sonner` (toasts), `node:test` для чистой логики.

## Global Constraints

- Тесты: `node --test --experimental-strip-types <file>` (Node 24). Скрипта `npm test` нет — запускать файл напрямую. Импорты в тестах — с расширением `.ts` (`allowImportingTsExtensions`).
- Файлы, тестируемые через `node:test`, **не** должны импортировать React/LiveKit — только чистая логика (паттерн `boardTools.ts` ↔ `boardTools.test.ts`).
- Все компоненты — `'use client'`.
- Стили — инлайн-объекты (как в текущих `PipCameras.tsx`/`BoardUi.tsx`), не Tailwind, чтобы точно попасть в токены хендоффа.
- Токены тем — строго из спеки §5 (`docs/superpowers/specs/2026-07-01-call-window-toolbar-design.md`).
- Сцена всегда тёмная: `STAGE_BG='#101113'`, `TILE_BG='#232427'`, `AVATAR='#5b5d63'`, `GLYPH='#3f4046'`.
- Все строки UI — на русском.
- Билд-проверка: `cd frontend && npx tsc --noEmit` должна проходить после каждой задачи, меняющей `.tsx`.
- Коммитить после каждой задачи.

---

### Task 1: `callTheme.ts` — токены тем + раскладка сцены (чистая логика, TDD)

**Files:**
- Create: `frontend/src/components/call/callTheme.ts`
- Test: `frontend/src/components/call/callTheme.test.ts`

**Interfaces:**
- Produces:
  - `type CallTheme = { panel; border; borderSoft; text; muted; hover; accent; accentBg; destructive; destructiveBg; success; successBg: string }`
  - `themeTokens(resolved: 'dark' | 'light'): CallTheme`
  - `type StageLayout = { mode: 'single' | 'grid'; columns: number }`
  - `layoutForCount(n: number): StageLayout`
  - Константы `STAGE_BG, TILE_BG, AVATAR, GLYPH: string`

- [ ] **Step 1: Write the failing test**

```ts
// frontend/src/components/call/callTheme.test.ts
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { themeTokens, layoutForCount } from './callTheme.ts'

test('layoutForCount: 0 и 1 участник → single', () => {
  assert.deepEqual(layoutForCount(0), { mode: 'single', columns: 1 })
  assert.deepEqual(layoutForCount(1), { mode: 'single', columns: 1 })
})

test('layoutForCount: 2 участника → сетка 2 колонки (кейс 1:1 урока)', () => {
  assert.deepEqual(layoutForCount(2), { mode: 'grid', columns: 2 })
})

test('layoutForCount: 3+ участников → сетка 3 колонки', () => {
  assert.deepEqual(layoutForCount(3), { mode: 'grid', columns: 3 })
  assert.deepEqual(layoutForCount(5), { mode: 'grid', columns: 3 })
})

test('themeTokens: dark и light дают разные panel/accent', () => {
  assert.equal(themeTokens('dark').panel, '#222222')
  assert.equal(themeTokens('light').panel, '#FFFFFF')
  assert.equal(themeTokens('dark').accent, '#6CA6E0')
  assert.equal(themeTokens('light').accent, '#1D4ED8')
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && node --test --experimental-strip-types src/components/call/callTheme.test.ts`
Expected: FAIL — `Cannot find module './callTheme.ts'`.

- [ ] **Step 3: Write minimal implementation**

```ts
// frontend/src/components/call/callTheme.ts
// Токены и раскладка окна звонка. Чистый модуль (без React/LiveKit) —
// тестируется через node:test.

export const STAGE_BG = '#101113'
export const TILE_BG = '#232427'
export const AVATAR = '#5b5d63'
export const GLYPH = '#3f4046'

export interface CallTheme {
  panel: string
  border: string
  borderSoft: string
  text: string
  muted: string
  hover: string
  accent: string
  accentBg: string
  destructive: string
  destructiveBg: string
  success: string
  successBg: string
}

const DARK: CallTheme = {
  panel: '#222222', border: '#2e2e2e', borderSoft: 'rgba(255,255,255,0.1)',
  text: '#E3E2E0', muted: '#979A9B', hover: '#3a3a3a',
  accent: '#6CA6E0', accentBg: 'rgba(108,166,224,0.16)',
  destructive: '#CD4945', destructiveBg: 'rgba(205,73,69,0.15)',
  success: '#4F9768', successBg: 'rgba(79,151,104,0.18)',
}

const LIGHT: CallTheme = {
  panel: '#FFFFFF', border: '#E1E1E4', borderSoft: 'rgba(27,28,31,0.1)',
  text: '#1B1C1F', muted: '#646670', hover: '#ECECEE',
  accent: '#1D4ED8', accentBg: 'rgba(29,78,216,0.08)',
  destructive: '#C92A2A', destructiveBg: 'rgba(201,42,42,0.08)',
  success: '#077A4E', successBg: 'rgba(7,122,78,0.1)',
}

export function themeTokens(resolved: 'dark' | 'light'): CallTheme {
  return resolved === 'dark' ? DARK : LIGHT
}

export interface StageLayout {
  mode: 'single' | 'grid'
  columns: number
}

// n<=1 → один центрированный tile; n===2 (кейс 1:1 урока) → 2 колонки;
// n>=3 → 3 колонки. Не хардкодим 3 — иначе 2 участника «проваливаются».
export function layoutForCount(n: number): StageLayout {
  if (n <= 1) return { mode: 'single', columns: 1 }
  if (n === 2) return { mode: 'grid', columns: 2 }
  return { mode: 'grid', columns: 3 }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd frontend && node --test --experimental-strip-types src/components/call/callTheme.test.ts`
Expected: PASS — 4 tests pass.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/call/callTheme.ts frontend/src/components/call/callTheme.test.ts
git commit -m "feat(call): theme tokens + participant layout helper"
```

---

### Task 2: `callChat.ts` + codec-тесты (чистая логика, TDD)

**Files:**
- Create: `frontend/src/components/call/callChat.ts`
- Test: `frontend/src/components/call/callChat.test.ts`

**Interfaces:**
- Produces:
  - `type ChatMessage = { from: string; text: string; mine: boolean }`
  - `encodeChat(from: string, text: string): Uint8Array`
  - `parseChatMessage(payload: Uint8Array): { from: string; text: string } | null` — возвращает `null`, если это не chat-сообщение (напр. board-open/close).

- [ ] **Step 1: Write the failing test**

```ts
// frontend/src/components/call/callChat.test.ts
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { encodeChat, parseChatMessage } from './callChat.ts'

test('encode → parse round-trip', () => {
  const bytes = encodeChat('Мария П.', 'Привет')
  assert.deepEqual(parseChatMessage(bytes), { from: 'Мария П.', text: 'Привет' })
})

test('parseChatMessage игнорирует не-chat сообщения (board-open)', () => {
  const board = new TextEncoder().encode(
    JSON.stringify({ type: 'board-open', board_token: 'abc' }),
  )
  assert.equal(parseChatMessage(board), null)
})

test('parseChatMessage возвращает null на битом payload', () => {
  assert.equal(parseChatMessage(new TextEncoder().encode('not json')), null)
})

test('encodeChat помечает сообщение type:chat', () => {
  const decoded = JSON.parse(new TextDecoder().decode(encodeChat('X', 'y')))
  assert.equal(decoded.type, 'chat')
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && node --test --experimental-strip-types src/components/call/callChat.test.ts`
Expected: FAIL — `Cannot find module './callChat.ts'`.

- [ ] **Step 3: Write minimal implementation**

```ts
// frontend/src/components/call/callChat.ts
// Чистый codec чат-сообщений. Чат и доска делят один DataChannel, поэтому
// дискриминант `type` обязателен, чтобы обработчики не глотали чужие сообщения.

export interface ChatMessage {
  from: string
  text: string
  mine: boolean
}

interface ChatWire {
  type: 'chat'
  from: string
  text: string
}

export function encodeChat(from: string, text: string): Uint8Array {
  const wire: ChatWire = { type: 'chat', from, text }
  return new TextEncoder().encode(JSON.stringify(wire))
}

export function parseChatMessage(payload: Uint8Array): { from: string; text: string } | null {
  let msg: unknown
  try {
    msg = JSON.parse(new TextDecoder().decode(payload))
  } catch {
    return null
  }
  if (
    typeof msg === 'object' && msg !== null &&
    (msg as ChatWire).type === 'chat' &&
    typeof (msg as ChatWire).from === 'string' &&
    typeof (msg as ChatWire).text === 'string'
  ) {
    return { from: (msg as ChatWire).from, text: (msg as ChatWire).text }
  }
  return null
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd frontend && node --test --experimental-strip-types src/components/call/callChat.test.ts`
Expected: PASS — 4 tests pass.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/call/callChat.ts frontend/src/components/call/callChat.test.ts
git commit -m "feat(call): chat message codec with type discriminant"
```

---

### Task 3: `useCallChat.ts` — хук чата на DataChannel

**Files:**
- Create: `frontend/src/components/call/useCallChat.ts`

**Interfaces:**
- Consumes: `encodeChat`, `parseChatMessage`, `ChatMessage` из `callChat.ts` (Task 2).
- Produces:
  - `useCallChat(opts: { chatOpen: boolean }): { messages: ChatMessage[]; send: (text: string) => void; unread: number }`
  - Хук ДОЛЖЕН вызываться в `CallRoomInner` (всегда смонтирован), не внутри тогглимой панели — иначе слушатель `DataReceived` отвалится и сообщения потеряются.

- [ ] **Step 1: Write implementation**

```ts
// frontend/src/components/call/useCallChat.ts
'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import { useRoomContext } from '@livekit/components-react'
import { RoomEvent, type RemoteParticipant } from 'livekit-client'
import { encodeChat, parseChatMessage, type ChatMessage } from './callChat'

// Живёт в CallRoomInner: слушатель не должен отваливаться при закрытом чате.
export function useCallChat({ chatOpen }: { chatOpen: boolean }) {
  const room = useRoomContext()
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [unread, setUnread] = useState(0)

  // Актуальный chatOpen для обработчика без пересоздания подписки.
  const chatOpenRef = useRef(chatOpen)
  useEffect(() => {
    chatOpenRef.current = chatOpen
    if (chatOpen) setUnread(0)
  }, [chatOpen])

  useEffect(() => {
    function onData(payload: Uint8Array, participant?: RemoteParticipant) {
      const parsed = parseChatMessage(payload)
      if (!parsed) return // board-open/close и прочее — не наше
      const from = participant?.name || participant?.identity || parsed.from
      setMessages((prev) => [...prev, { from, text: parsed.text, mine: false }])
      if (!chatOpenRef.current) setUnread((u) => u + 1)
    }
    room.on(RoomEvent.DataReceived, onData)
    return () => { room.off(RoomEvent.DataReceived, onData) }
  }, [room])

  const send = useCallback((text: string) => {
    const trimmed = text.trim()
    if (!trimmed) return
    const from = room.localParticipant.name || room.localParticipant.identity
    room.localParticipant
      .publishData(encodeChat(from, trimmed), { reliable: true })
      .catch(() => {})
    setMessages((prev) => [...prev, { from: 'Вы', text: trimmed, mine: true }])
  }, [room])

  return { messages, send, unread }
}
```

- [ ] **Step 2: Type-check**

Run: `cd frontend && npx tsc --noEmit`
Expected: PASS (no errors referencing `useCallChat.ts`).

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/call/useCallChat.ts
git commit -m "feat(call): DataChannel chat hook (lives in always-mounted parent)"
```

---

### Task 4: `CallTile.tsx` — один tile участника

**Files:**
- Create: `frontend/src/components/call/CallTile.tsx`

**Interfaces:**
- Consumes: `TILE_BG, AVATAR, GLYPH` из `callTheme.ts` (Task 1).
- Produces: `CallTile({ trackRef, variant }: { trackRef: TrackReferenceOrPlaceholder; variant: 'speaker' | 'grid' })`

- [ ] **Step 1: Write implementation**

```tsx
// frontend/src/components/call/CallTile.tsx
'use client'

import {
  VideoTrack,
  isTrackReference,
  useTrackMutedIndicator,
  type TrackReferenceOrPlaceholder,
} from '@livekit/components-react'
import { Track } from 'livekit-client'
import { TILE_BG, AVATAR, GLYPH } from './callTheme'

interface Props {
  trackRef: TrackReferenceOrPlaceholder
  variant: 'speaker' | 'grid'
}

export function CallTile({ trackRef, variant }: Props) {
  const speaker = variant === 'speaker'
  const name = trackRef.participant.name || trackRef.participant.identity
  const { isMuted } = useTrackMutedIndicator({
    participant: trackRef.participant,
    source: Track.Source.Microphone,
  })
  const hasVideo = isTrackReference(trackRef)
  const avatarSize = speaker ? 168 : 62
  const glyphSize = speaker ? 94 : 34

  return (
    <div
      style={{
        position: 'relative',
        width: '100%',
        height: '100%',
        borderRadius: speaker ? 0 : 12,
        overflow: 'hidden',
        background: TILE_BG,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      {hasVideo ? (
        <VideoTrack
          trackRef={trackRef}
          style={{ width: '100%', height: '100%', objectFit: 'cover' }}
        />
      ) : (
        <div
          style={{
            width: avatarSize,
            height: avatarSize,
            borderRadius: '50%',
            background: AVATAR,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <svg width={glyphSize} height={glyphSize} viewBox="0 0 24 24" fill={GLYPH}>
            <circle cx="12" cy="8" r="4.1" />
            <path d="M4 21c0-4.4 3.6-8 8-8s8 3.6 8 8Z" />
          </svg>
        </div>
      )}

      {/* Плашка имени — сверху-слева (низ занят тулбаром) */}
      <div
        style={{
          position: 'absolute',
          left: speaker ? 18 : 10,
          top: speaker ? 18 : 10,
          display: 'flex',
          alignItems: 'center',
          gap: speaker ? 7 : 5,
          background: 'rgba(14,14,15,0.68)',
          padding: speaker ? '6px 12px' : '4px 9px',
          borderRadius: speaker ? 9 : 7,
        }}
      >
        {isMuted && (
          <svg
            width={speaker ? 14 : 12}
            height={speaker ? 14 : 12}
            viewBox="0 0 24 24"
            fill="none"
            stroke="#fff"
            strokeWidth={speaker ? 1.8 : 2}
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <path d="M9 9v3a3 3 0 0 0 4.6 2.5" />
            <path d="M15 9.34V5a3 3 0 0 0-5.94-.6" />
            <path d="M19 10v1a7 7 0 0 1-.32 2.1" />
            <path d="M5 10v1a7 7 0 0 0 11.3 5.5" />
            <path d="M12 18v4" />
            <path d="M8 22h8" />
            <path d="M2 2l20 20" />
          </svg>
        )}
        <span style={{ color: '#fff', fontSize: speaker ? 13 : 12, fontWeight: 500 }}>
          {name}
        </span>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Type-check**

Run: `cd frontend && npx tsc --noEmit`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/call/CallTile.tsx
git commit -m "feat(call): participant tile (video/avatar + top-left name chip)"
```

---

### Task 5: `CallStage.tsx` — раскладка участников

**Files:**
- Create: `frontend/src/components/call/CallStage.tsx`

**Interfaces:**
- Consumes: `layoutForCount, STAGE_BG` (Task 1), `CallTile` (Task 4).
- Produces: `CallStage()` — без пропсов; читает треки из LiveKit-контекста.

- [ ] **Step 1: Write implementation**

```tsx
// frontend/src/components/call/CallStage.tsx
'use client'

import { useTracks } from '@livekit/components-react'
import { Track } from 'livekit-client'
import { layoutForCount, STAGE_BG } from './callTheme'
import { CallTile } from './CallTile'

export function CallStage() {
  const tracks = useTracks(
    [
      { source: Track.Source.Camera, withPlaceholder: true },
      { source: Track.Source.ScreenShare, withPlaceholder: false },
    ],
    { onlySubscribed: false },
  )

  // Дедуп placeholder+реальный трек одного participant+source (гонка
  // перепубликации камеры), предпочитая реальный трек — как в PipCameras.
  const byKey = new Map<string, (typeof tracks)[number]>()
  for (const t of tracks) {
    const key = `${t.participant.identity}-${t.source}`
    const existing = byKey.get(key)
    if (!existing || ('publication' in t && t.publication)) byKey.set(key, t)
  }
  const unique = [...byKey.values()]
  const layout = layoutForCount(unique.length)

  return (
    <div style={{ width: '100%', height: '100%', background: STAGE_BG }}>
      {layout.mode === 'single' ? (
        <div style={{ width: '100%', height: '100%' }}>
          {unique[0] && <CallTile trackRef={unique[0]} variant="speaker" />}
        </div>
      ) : (
        <div
          style={{
            width: '100%',
            height: '100%',
            display: 'grid',
            gridTemplateColumns: `repeat(${layout.columns}, 1fr)`,
            gap: 10,
            padding: 18,
            boxSizing: 'border-box',
          }}
        >
          {unique.map((t) => (
            <CallTile key={`${t.participant.identity}-${t.source}`} trackRef={t} variant="grid" />
          ))}
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 2: Type-check**

Run: `cd frontend && npx tsc --noEmit`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/call/CallStage.tsx
git commit -m "feat(call): stage layout (single/grid via layoutForCount)"
```

---

### Task 6: `DeviceSettings.tsx` — выбор камеры/микрофона

**Files:**
- Create: `frontend/src/components/call/DeviceSettings.tsx`

**Interfaces:**
- Consumes: `CallTheme` (Task 1).
- Produces: `DeviceSettings({ theme }: { theme: CallTheme })`

- [ ] **Step 1: Write implementation**

```tsx
// frontend/src/components/call/DeviceSettings.tsx
'use client'

import { useMediaDeviceSelect } from '@livekit/components-react'
import type { CallTheme } from './callTheme'

function DeviceGroup({
  kind, label, theme,
}: { kind: 'audioinput' | 'videoinput'; label: string; theme: CallTheme }) {
  const { devices, activeDeviceId, setActiveMediaDevice } = useMediaDeviceSelect({ kind })
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 4, padding: '4px 6px' }}>
      <div style={{ fontSize: 11, fontWeight: 600, color: theme.muted }}>{label}</div>
      {devices.map((d) => {
        const active = d.deviceId === activeDeviceId
        return (
          <button
            key={d.deviceId}
            onClick={() => setActiveMediaDevice(d.deviceId)}
            style={{
              textAlign: 'left',
              border: 'none',
              background: active ? theme.accentBg : 'transparent',
              color: active ? theme.accent : theme.text,
              fontSize: 12,
              padding: '7px 9px',
              borderRadius: 8,
              cursor: 'pointer',
              fontFamily: 'inherit',
              whiteSpace: 'nowrap',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              maxWidth: 220,
            }}
          >
            {d.label || 'Устройство'}
          </button>
        )
      })}
    </div>
  )
}

export function DeviceSettings({ theme }: { theme: CallTheme }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
      <DeviceGroup kind="audioinput" label="Микрофон" theme={theme} />
      <div style={{ height: 1, background: theme.border, margin: '2px 6px' }} />
      <DeviceGroup kind="videoinput" label="Камера" theme={theme} />
    </div>
  )
}
```

- [ ] **Step 2: Type-check**

Run: `cd frontend && npx tsc --noEmit`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/call/DeviceSettings.tsx
git commit -m "feat(call): device settings (camera/mic select) for More menu"
```

---

### Task 7: `CallChat.tsx` — правая панель чата

**Files:**
- Create: `frontend/src/components/call/CallChat.tsx`

**Interfaces:**
- Consumes: `ChatMessage` (Task 2), `CallTheme` (Task 1).
- Produces: `CallChat({ theme, messages, onSend, onClose }: { theme: CallTheme; messages: ChatMessage[]; onSend: (text: string) => void; onClose: () => void })`

- [ ] **Step 1: Write implementation**

```tsx
// frontend/src/components/call/CallChat.tsx
'use client'

import { useEffect, useRef, useState } from 'react'
import type { CallTheme } from './callTheme'
import type { ChatMessage } from './callChat'

interface Props {
  theme: CallTheme
  messages: ChatMessage[]
  onSend: (text: string) => void
  onClose: () => void
}

export function CallChat({ theme, messages, onSend, onClose }: Props) {
  const [draft, setDraft] = useState('')
  const listRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (listRef.current) listRef.current.scrollTop = listRef.current.scrollHeight
  }, [messages])

  function submit() {
    if (!draft.trim()) return
    onSend(draft)
    setDraft('')
  }

  return (
    <div
      style={{
        position: 'absolute', top: 0, right: 0, bottom: 0, width: 280,
        background: theme.panel, borderLeft: `1px solid ${theme.border}`,
        display: 'flex', flexDirection: 'column', zIndex: 12, color: theme.text,
        fontFamily: '-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif',
      }}
    >
      <div
        style={{
          display: 'flex', alignItems: 'center', justifyContent: 'space-between',
          padding: '13px 14px', borderBottom: `1px solid ${theme.border}`,
        }}
      >
        <span style={{ fontWeight: 600, fontSize: 14 }}>Чат</span>
        <button
          onClick={onClose}
          aria-label="Закрыть чат"
          style={{
            width: 26, height: 26, borderRadius: 7, border: 'none',
            background: 'transparent', color: theme.muted, cursor: 'pointer',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M18 6L6 18" /><path d="M6 6l12 12" />
          </svg>
        </button>
      </div>

      <div ref={listRef} style={{ flex: 1, overflowY: 'auto', padding: '12px 14px', display: 'flex', flexDirection: 'column', gap: 10 }}>
        {messages.map((m, i) => (
          <div key={i} style={{ alignSelf: m.mine ? 'flex-end' : 'flex-start', maxWidth: '85%' }}>
            <div style={{ fontSize: 10.5, color: theme.muted, marginBottom: 2, textAlign: m.mine ? 'right' : 'left' }}>
              {m.from}
            </div>
            <div style={{ fontSize: 13, lineHeight: 1.4, padding: '7px 10px', borderRadius: 10, background: m.mine ? theme.accentBg : theme.hover, color: theme.text }}>
              {m.text}
            </div>
          </div>
        ))}
      </div>

      <div style={{ display: 'flex', gap: 8, padding: '11px 12px', borderTop: `1px solid ${theme.border}` }}>
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); submit() } }}
          placeholder="Написать сообщение…"
          style={{
            flex: 1, border: `1px solid ${theme.border}`,
            background: theme.panel === '#FFFFFF' ? '#F8F8F9' : '#1a1a1a',
            color: theme.text, borderRadius: 8, padding: '8px 10px', fontSize: 13,
            fontFamily: 'inherit', outline: 'none',
          }}
        />
        <button
          onClick={submit}
          aria-label="Отправить"
          style={{
            width: 34, height: 34, borderRadius: 8, border: 'none',
            background: theme.accent, color: '#fff', cursor: 'pointer', flexShrink: 0,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M22 2L11 13" /><path d="M22 2l-7 20-4-9-9-4 20-7Z" />
          </svg>
        </button>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Type-check**

Run: `cd frontend && npx tsc --noEmit`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/call/CallChat.tsx
git commit -m "feat(call): chat panel (docked right, enter-to-send, autoscroll)"
```

---

### Task 8: `CallToolbar.tsx` — компактный нижний тулбар

**Files:**
- Create: `frontend/src/components/call/CallToolbar.tsx`

**Interfaces:**
- Consumes: `themeTokens` (Task 1), `DeviceSettings` (Task 6), `useLocalParticipant` (LiveKit), `useTheme` (next-themes).
- Produces:
  ```ts
  CallToolbar(props: {
    role: 'tutor' | 'guest'
    inviteUrl?: string
    boardActive: boolean
    chatActive: boolean
    chatUnread: number
    onToggleBoard: () => void
    onToggleChat: () => void
    onLeave: () => void
  })
  ```

- [ ] **Step 1: Write implementation**

```tsx
// frontend/src/components/call/CallToolbar.tsx
'use client'

import { useEffect, useRef, useState, type ReactNode } from 'react'
import { useLocalParticipant } from '@livekit/components-react'
import { useTheme } from 'next-themes'
import { themeTokens } from './callTheme'
import { DeviceSettings } from './DeviceSettings'

const FONT = '-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif'

// Иконки 15×15, stroke=currentColor, strokeWidth 1.8.
const ICONS = {
  linkChain: (<><path d="M9 15l6-6" /><path d="M8 11L6.5 12.5a3.5 3.5 0 0 0 5 5L13 16" /><path d="M16 13l1.5-1.5a3.5 3.5 0 0 0-5-5L11 8" /></>),
  check: <path d="M20 6L9 17l-5-5" />,
  micOn: (<><path d="M12 2a3 3 0 0 0-3 3v6a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z" /><path d="M19 10v1a7 7 0 0 1-14 0v-1" /><path d="M12 18v4" /><path d="M8 22h8" /></>),
  micOff: (<><path d="M9 9v3a3 3 0 0 0 4.6 2.5" /><path d="M15 9.34V5a3 3 0 0 0-5.94-.6" /><path d="M19 10v1a7 7 0 0 1-.32 2.1" /><path d="M5 10v1a7 7 0 0 0 11.3 5.5" /><path d="M12 18v4" /><path d="M8 22h8" /><path d="M2 2l20 20" /></>),
  camOn: (<><path d="M15 8l5-3v14l-5-3" /><rect x="2" y="6" width="13" height="12" rx="2" /></>),
  camOff: (<><path d="M2 2l20 20" /><path d="M15 8l5-3v14l-2.2-1.32" /><path d="M2 8v10a2 2 0 0 0 2 2h9.5" /><path d="M2 6.5A2 2 0 0 1 4 5h9a2 2 0 0 1 2 2v2.5" /></>),
  share: (<><rect x="3" y="3" width="18" height="18" rx="3" /><path d="M12 16V8" /><path d="M9 11l3-3 3 3" /></>),
  board: (<><path d="M12 4H6a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-6" /><path d="M18.4 3.6a2.1 2.1 0 1 1 3 3L12 15.5l-4 1 1-4Z" /></>),
  chat: <path d="M21 11.5a8.5 8.5 0 0 1-8.5 8.5 8.4 8.4 0 0 1-3.8-.9L3 21l1.9-5.7a8.4 8.4 0 0 1-.9-3.8 8.5 8.5 0 0 1 8.5-8.5h.5a8.4 8.4 0 0 1 8 8v.5Z" />,
  more: (<><circle cx="5" cy="12" r="1.6" /><circle cx="12" cy="12" r="1.6" /><circle cx="19" cy="12" r="1.6" /></>),
  leave: (<><path d="M9 21H6a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3" /><path d="M16 17l5-5-5-5" /><path d="M21 12H9" /></>),
}

function Icon({ children }: { children: ReactNode }) {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      {children}
    </svg>
  )
}

interface Props {
  role: 'tutor' | 'guest'
  inviteUrl?: string
  boardActive: boolean
  chatActive: boolean
  chatUnread: number
  onToggleBoard: () => void
  onToggleChat: () => void
  onLeave: () => void
}

export function CallToolbar({
  role, inviteUrl, boardActive, chatActive, chatUnread,
  onToggleBoard, onToggleChat, onLeave,
}: Props) {
  const { resolvedTheme } = useTheme()
  const c = themeTokens(resolvedTheme === 'dark' ? 'dark' : 'light')
  const { localParticipant, isMicrophoneEnabled, isCameraEnabled, isScreenShareEnabled } =
    useLocalParticipant()

  const [copied, setCopied] = useState(false)
  const [moreOpen, setMoreOpen] = useState(false)
  const [confirmLeave, setConfirmLeave] = useState(false)
  const copyTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => () => { if (copyTimer.current) clearTimeout(copyTimer.current) }, [])

  function copyLink() {
    if (!inviteUrl) return
    try { navigator.clipboard.writeText(inviteUrl) } catch {}
    setCopied(true)
    if (copyTimer.current) clearTimeout(copyTimer.current)
    copyTimer.current = setTimeout(() => setCopied(false), 2200)
  }

  const btnBase = (opts: { active?: boolean; danger?: boolean; copied?: boolean }): React.CSSProperties => {
    let bg = 'transparent'
    let color = c.text
    if (opts.danger) { bg = c.destructiveBg; color = c.destructive }
    else if (opts.copied) { bg = c.successBg; color = c.success }
    else if (opts.active) { bg = c.accentBg; color = c.accent }
    return {
      position: 'relative', width: 32, height: 32, minWidth: 32,
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      padding: 0, border: 'none', borderRadius: 8, background: bg, color,
      cursor: 'pointer', flexShrink: 0, transition: 'background .15s,color .15s',
    }
  }

  return (
    <>
      {/* Тост копирования */}
      {copied && (
        <div style={{
          position: 'absolute', top: 16, left: '50%', transform: 'translateX(-50%)',
          background: 'rgba(20,20,21,0.9)', color: '#fff', padding: '8px 14px',
          borderRadius: 10, display: 'flex', alignItems: 'center', gap: 8,
          fontSize: 13, fontWeight: 500, zIndex: 60, fontFamily: FONT,
        }}>
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="#4F9768" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round">
            <path d="M20 6L9 17l-5-5" />
          </svg>
          Ссылка на звонок скопирована
        </div>
      )}

      {/* Меню «Ещё» (вверх) */}
      {moreOpen && (
        <div style={{
          position: 'absolute', right: 22, bottom: 72, background: c.panel,
          border: `1px solid ${c.border}`, borderRadius: 12, padding: 6,
          display: 'flex', flexDirection: 'column', minWidth: 240,
          boxShadow: '0 16px 34px rgba(0,0,0,0.3)', zIndex: 20, fontFamily: FONT,
        }}>
          <DeviceSettings theme={c} />
        </div>
      )}

      {/* Модалка выхода */}
      {confirmLeave && (
        <div style={{
          position: 'absolute', inset: 0, background: 'rgba(0,0,0,0.5)',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          zIndex: 70, fontFamily: FONT,
        }}>
          <div style={{ width: 320, background: c.panel, borderRadius: 14, padding: 20, boxShadow: '0 20px 50px rgba(0,0,0,0.35)' }}>
            <div style={{ fontSize: 16, fontWeight: 700, color: c.text, marginBottom: 6 }}>Покинуть звонок?</div>
            <div style={{ fontSize: 13, color: c.muted, marginBottom: 18, lineHeight: 1.4 }}>Вы сможете вернуться по той же ссылке.</div>
            <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end' }}>
              <button onClick={() => setConfirmLeave(false)} style={{ border: `1px solid ${c.border}`, background: 'transparent', color: c.text, borderRadius: 9, padding: '8px 16px', fontSize: 13, fontWeight: 600, cursor: 'pointer', fontFamily: FONT }}>Отмена</button>
              <button onClick={onLeave} style={{ border: 'none', background: c.destructive, color: '#fff', borderRadius: 9, padding: '8px 16px', fontSize: 13, fontWeight: 600, cursor: 'pointer', fontFamily: FONT }}>Покинуть</button>
            </div>
          </div>
        </div>
      )}

      {/* Сам тулбар */}
      <div style={{
        position: 'absolute', left: '50%', bottom: 14, transform: 'translateX(-50%)',
        display: 'flex', alignItems: 'center', gap: 2, padding: '5px 6px',
        borderRadius: 10, background: c.panel, border: `1px solid ${c.border}`,
        boxShadow: '0 6px 18px rgba(0,0,0,0.22)', zIndex: 10, fontFamily: FONT,
      }}>
        {role === 'tutor' && inviteUrl && (
          <button onClick={copyLink} title="Скопировать ссылку" style={btnBase({ copied })}>
            <Icon>{copied ? ICONS.check : ICONS.linkChain}</Icon>
          </button>
        )}
        <button
          onClick={() => localParticipant.setMicrophoneEnabled(!isMicrophoneEnabled)}
          title="Микрофон" style={btnBase({})}
        >
          <Icon>{isMicrophoneEnabled ? ICONS.micOn : ICONS.micOff}</Icon>
        </button>
        <button
          onClick={() => localParticipant.setCameraEnabled(!isCameraEnabled)}
          title="Камера" style={btnBase({})}
        >
          <Icon>{isCameraEnabled ? ICONS.camOn : ICONS.camOff}</Icon>
        </button>
        <button
          onClick={() => localParticipant.setScreenShareEnabled(!isScreenShareEnabled)}
          title="Демонстрация экрана" style={btnBase({ active: isScreenShareEnabled })}
        >
          <Icon>{ICONS.share}</Icon>
        </button>
        {role === 'tutor' && (
          <button onClick={() => { setMoreOpen(false); onToggleBoard() }} title="Доска" style={btnBase({ active: boardActive })}>
            <Icon>{ICONS.board}</Icon>
          </button>
        )}
        <button onClick={() => { setMoreOpen(false); onToggleChat() }} title="Чат" style={btnBase({ active: chatActive })}>
          <Icon>{ICONS.chat}</Icon>
          {chatUnread > 0 && !chatActive && (
            <span style={{
              position: 'absolute', top: -2, right: -2, minWidth: 15, height: 15,
              padding: '0 4px', borderRadius: 8, background: c.destructive, color: '#fff',
              fontSize: 9, fontWeight: 700, display: 'flex', alignItems: 'center', justifyContent: 'center',
            }}>{chatUnread}</span>
          )}
        </button>
        <button onClick={() => setMoreOpen((v) => !v)} title="Ещё" style={btnBase({ active: moreOpen })}>
          <Icon>{ICONS.more}</Icon>
        </button>
        <button onClick={() => setConfirmLeave(true)} title="Покинуть звонок" style={btnBase({ danger: true })}>
          <Icon>{ICONS.leave}</Icon>
        </button>
      </div>
    </>
  )
}
```

- [ ] **Step 2: Type-check**

Run: `cd frontend && npx tsc --noEmit`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/call/CallToolbar.tsx
git commit -m "feat(call): compact bottom toolbar (media/board/chat/more/leave)"
```

---

### Task 9: Интеграция в `CallRoom.tsx`

**Files:**
- Modify: `frontend/src/components/call/CallRoom.tsx`

**Interfaces:**
- Consumes: `CallStage` (Task 5), `CallToolbar` (Task 8), `CallChat` (Task 7), `useCallChat` (Task 3), `themeTokens` (Task 1).
- Produces: `CallRoomProps` теперь включает `inviteUrl?: string`.

**Detail:** `CallRoomInner` заменяет `VideoGrid`/`BoardToggleButton` на новую сцену/тулбар/чат, поднимает `chatOpen`, монтирует `useCallChat`, добавляет `onLeave = room.disconnect`. Board-обработчик уже игнорирует не-board сообщения (`msg.type` проверяется) — чат-сообщения туда не попадут.

- [ ] **Step 1: Replace imports block (lines 13–18)**

Заменить:
```tsx
import { VideoGrid } from './VideoGrid'
import { PipCameras } from './PipCameras'
import { BoardToggleButton } from './BoardToggleButton'
import { TldrawCanvas } from '@/components/whiteboard/TldrawCanvas'
import { whiteboardApi } from '@/lib/api/whiteboard'
import type { BoardWithPages } from '@/types/api'
```
на:
```tsx
import { useTheme } from 'next-themes'
import { PipCameras } from './PipCameras'
import { CallStage } from './CallStage'
import { CallToolbar } from './CallToolbar'
import { CallChat } from './CallChat'
import { useCallChat } from './useCallChat'
import { themeTokens } from './callTheme'
import { TldrawCanvas } from '@/components/whiteboard/TldrawCanvas'
import { whiteboardApi } from '@/lib/api/whiteboard'
import type { BoardWithPages } from '@/types/api'
```

- [ ] **Step 2: Extend `CallRoomInnerProps` and add UI state**

Заменить сигнатуру и начало `CallRoomInner` (строки 29–38):
```tsx
interface CallRoomInnerProps {
  courseId?: string
  role: 'tutor' | 'guest'
}

function CallRoomInner({ courseId, role }: CallRoomInnerProps) {
  const room = useRoomContext()
  const [mode, setMode] = useState<Mode>('call')
  const [boardLoading, setBoardLoading] = useState(false)
  const [activePageId, setActivePageId] = useState<string | null>(null)
```
на:
```tsx
interface CallRoomInnerProps {
  courseId?: string
  role: 'tutor' | 'guest'
  inviteUrl?: string
}

function CallRoomInner({ courseId, role, inviteUrl }: CallRoomInnerProps) {
  const room = useRoomContext()
  const { resolvedTheme } = useTheme()
  const theme = themeTokens(resolvedTheme === 'dark' ? 'dark' : 'light')
  const [mode, setMode] = useState<Mode>('call')
  const [boardLoading, setBoardLoading] = useState(false)
  const [activePageId, setActivePageId] = useState<string | null>(null)
  const [chatOpen, setChatOpen] = useState(false)
  const { messages, send, unread } = useCallChat({ chatOpen })
```

> Примечание: `boardLoading` больше не отображается кнопкой, но остаётся флагом защиты от повторного клика в `handleToggle`. Оставить как есть.

- [ ] **Step 3: Replace the render return (lines 164–191)**

Заменить весь `return (...)` в `CallRoomInner` на:
```tsx
  return (
    <div style={{ position: 'relative', width: '100%', height: '100%', overflow: 'hidden' }}>
      {/* Область сцены/доски ужимается при открытом чате */}
      <div
        style={{
          position: 'absolute', top: 0, left: 0, bottom: 0,
          right: chatOpen ? 280 : 0, transition: 'right .25s ease',
        }}
      >
        {mode === 'call' && <CallStage />}

        {mode === 'board' && activeBoard && resolvedPageId && (
          <TldrawCanvas
            page={currentPage}
            boardId={activeBoard.id}
            pages={activeBoard.pages}
            activePageId={resolvedPageId}
            onSelectPage={setActivePageId}
            token={activeBoardToken}
            courseId={role === 'tutor' ? courseId ?? undefined : undefined}
            isGuest={role === 'guest'}
          />
        )}

        {mode === 'board' && <PipCameras />}
      </div>

      {chatOpen && (
        <CallChat
          theme={theme}
          messages={messages}
          onSend={send}
          onClose={() => setChatOpen(false)}
        />
      )}

      <CallToolbar
        role={role}
        inviteUrl={inviteUrl}
        boardActive={mode === 'board'}
        chatActive={chatOpen}
        chatUnread={unread}
        onToggleBoard={handleToggle}
        onToggleChat={() => setChatOpen((v) => !v)}
        onLeave={() => room.disconnect()}
      />
    </div>
  )
```

- [ ] **Step 4: Extend public props and pass `inviteUrl` down**

В `CallRoomProps` (строки 196–203) добавить `inviteUrl?: string`, и передать его в `CallRoomInner`. Заменить:
```tsx
export interface CallRoomProps {
  courseId?: string
  serverUrl: string
  token: string
  role: 'tutor' | 'guest'
  enableMedia?: boolean
  onDisconnected: () => void
}

export function CallRoom({
  courseId,
  serverUrl,
  token,
  role,
  enableMedia = false,
  onDisconnected,
}: CallRoomProps) {
```
на:
```tsx
export interface CallRoomProps {
  courseId?: string
  serverUrl: string
  token: string
  role: 'tutor' | 'guest'
  enableMedia?: boolean
  inviteUrl?: string
  onDisconnected: () => void
}

export function CallRoom({
  courseId,
  serverUrl,
  token,
  role,
  enableMedia = false,
  inviteUrl,
  onDisconnected,
}: CallRoomProps) {
```
и в JSX ниже заменить `<CallRoomInner courseId={courseId} role={role} />` на `<CallRoomInner courseId={courseId} role={role} inviteUrl={inviteUrl} />`.

- [ ] **Step 5: Type-check**

Run: `cd frontend && npx tsc --noEmit`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/call/CallRoom.tsx
git commit -m "feat(call): wire stage/toolbar/chat into CallRoom, add inviteUrl"
```

---

### Task 10: Обновить страницу репетитора + удалить старые файлы

**Files:**
- Modify: `frontend/src/app/(call)/lessons/[id]/call/page.tsx`
- Delete: `frontend/src/components/call/BoardToggleButton.tsx`
- Delete: `frontend/src/components/call/VideoGrid.tsx`

**Interfaces:**
- Consumes: `CallRoomProps.inviteUrl` (Task 9).

- [ ] **Step 1: Rewrite `call/page.tsx`**

Заменить файл `frontend/src/app/(call)/lessons/[id]/call/page.tsx` целиком на:
```tsx
'use client'

import { useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import '@livekit/components-styles'

import { callsApi, type RoomTokenResponse } from '@/lib/api/calls'
import { lessonsApi } from '@/lib/api/lessons'
import { Button } from '@/components/ui/button'
import { CallRoom } from '@/components/call/CallRoom'

export default function CallPage() {
  const { id } = useParams<{ id: string }>()
  const router = useRouter()

  const [room, setRoom] = useState<RoomTokenResponse | null>(null)
  const [courseId, setCourseId] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  // Auto-start on mount — репетитор уже начал звонок из календаря.
  // start-room идемпотентен, безопасно перезапускать при рефреше.
  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        await callsApi.startRoom(id)
        const [data, lesson] = await Promise.all([
          callsApi.getRoomToken(id),
          lessonsApi.get(id).catch(() => null),
        ])
        if (cancelled) return
        if (lesson?.course_id) setCourseId(lesson.course_id)
        setRoom(data)
      } catch {
        if (!cancelled) setError('Не удалось запустить урок')
      }
    })()
    return () => { cancelled = true }
  }, [id])

  async function handleDisconnected() {
    try { await callsApi.endRoom(id) } catch {}
    router.back()
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center h-[80vh] gap-4">
        <p className="text-destructive">{error}</p>
        <Button variant="outline" onClick={() => router.back()}>Назад</Button>
      </div>
    )
  }

  if (!room) {
    return (
      <div className="flex items-center justify-center h-[80vh]">
        <p className="text-muted-foreground">Подключение...</p>
      </div>
    )
  }

  const inviteUrl = `${window.location.origin}/join/${id}`

  return (
    <div style={{ height: '100dvh', position: 'relative' }}>
      <CallRoom
        courseId={courseId ?? undefined}
        serverUrl={room.server_url}
        token={room.token}
        role="tutor"
        inviteUrl={inviteUrl}
        onDisconnected={handleDisconnected}
      />
    </div>
  )
}
```

> Примечание: `window.location.origin` безопасен — файл `'use client'`, а к моменту рендера `room` уже загружен на клиенте.

- [ ] **Step 2: Delete obsolete components**

```bash
git rm frontend/src/components/call/BoardToggleButton.tsx frontend/src/components/call/VideoGrid.tsx
```

- [ ] **Step 3: Verify no dangling references**

Run: `cd frontend && grep -rn "BoardToggleButton\|VideoGrid" src`
Expected: no output (все ссылки убраны).

- [ ] **Step 4: Type-check + build**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: tsc PASS; `next build` завершается успешно.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/app/(call)/lessons/[id]/call/page.tsx
git commit -m "feat(call): pass inviteUrl, drop floating copy button + old ControlBar"
```

---

### Task 11: Полный прогон тестов + ручная проверка

**Files:** нет изменений (верификация).

- [ ] **Step 1: Run all frontend unit tests**

Run:
```bash
cd frontend && node --test --experimental-strip-types \
  src/components/call/callTheme.test.ts \
  src/components/call/callChat.test.ts \
  src/components/whiteboard/boardTools.test.ts
```
Expected: все тесты PASS (8 новых + 5 существующих).

- [ ] **Step 2: Manual smoke (описание для ревьюера)**

Запустить `npm run dev`, открыть урок → звонок. Проверить:
  1. Нижний тулбар по центру; mic/камера тумблятся и иконка меняется на «off/slashed».
  2. Кнопка «Ссылка» (только tutor) копирует URL, показывает тост ~2.2s и галочку.
  3. «Доска» открывает tldraw (существующий BoardUi сверху, PiP справа), кнопка подсвечена.
  4. «Чат» открывает панель справа, сцена ужимается на 280px; Enter отправляет; при закрытом чате входящее даёт бейдж.
  5. «Ещё» открывает список устройств; выбор переключает камеру/микрофон.
  6. «Покинуть» → модалка → «Покинуть» отключает и роутит назад.
  7. Переключение темы приложения меняет chrome тулбара/панелей; сцена остаётся тёмной.
  8. Гостевой вход (`/join/<id>` в другом окне): тулбар без «Ссылка»/«Доска».

- [ ] **Step 3: Final commit (if any tweaks)**

```bash
git add -A && git commit -m "test(call): verify full toolbar redesign"
```

---

## Self-Review

**Spec coverage:**
- §1 замена трёх плавающих элементов → Tasks 8,9,10 ✅
- §2 чат (DataChannel) → Tasks 2,3,7,9 ✅; тема (next-themes) → Tasks 1,8,9 ✅; «Ещё»=устройства → Task 6 ✅; share screen → Task 8 ✅; нет ended-экрана → Task 9 (`onLeave=room.disconnect`) ✅
- §4 архитектура/файлы → Tasks 1–10 покрывают все семь новых файлов (`callTheme`, `callChat`, `useCallChat`, `CallTile`, `CallStage`, `DeviceSettings`, `CallChat`, `CallToolbar`) и удаление `BoardToggleButton`/`VideoGrid` ✅ (примечание: спека называла файл токенов `callTheme.ts`, codec вынесен в отдельный `callChat.ts` для тестируемости без React — оправданное уточнение)
- §5 контракты (`layoutForCount`, чат-хук в родителе, tile top-left chip) → Tasks 1,3,4 ✅
- §6 один DataChannel + дискриминант в обоих обработчиках → Task 2 (parseChatMessage возвращает null на board) + существующий board-обработчик проверяет `msg.type` (Task 9 не трогает его) ✅
- §7 роли → Task 8 (условный рендер «Ссылка»/«Доска») ✅
- §9 тесты (`layoutForCount`, codec) → Tasks 1,2 ✅

**Placeholder scan:** нет TBD/TODO; код полный в каждом шаге.

**Type consistency:** `themeTokens`/`CallTheme`/`layoutForCount`/`StageLayout` (Task 1) → используются в 4,5,6,7,8,9 с теми же именами; `ChatMessage`/`encodeChat`/`parseChatMessage` (Task 2) → 3,7; `useCallChat` возвращает `{messages,send,unread}` (Task 3) → потребляется в Task 9 теми же именами; `CallToolbar` пропсы (Task 8) → передаются в Task 9 совпадающими именами. ✅

**Гэпы:** не найдено.
