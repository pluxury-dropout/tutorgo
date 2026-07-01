# Board Custom UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Заменить дефолтный UI tldraw на кастомный по мокапу (тема paper): верхний док + поповер настроек; PiP-видео звонка — круглое, сверху-справа.

**Architecture:** `<Tldraw hideUi>` прячет дефолтный UI; кастомный `BoardUi` рендерится как child (в контексте редактора, `useEditor()`+`track()`). Чистые маппинги вынесены в `boardTools.ts` и покрыты тестом. Видео — рестайл существующего `PipCameras`.

**Tech Stack:** Next.js/React, TypeScript, @tldraw/tldraw ^5.1.0, LiveKit, node:test.

## Global Constraints

- Тема только **paper**, константы захардкожены (без переключения тем).
- Обёртка кастомного UI = `pointer-events:none`; каждый интерактивный узел = `pointer-events:auto`.
- Стили применять к next-shapes **и** к selected-shapes.
- Тесты — `node:test`, без фреймворков; TS-strip Node 24 (`allowImportingTsExtensions`).
- Русский язык в UI-строках и комментариях.

---

## File Structure

- Create `frontend/src/components/whiteboard/boardTools.ts` — чистые маппинги/хелперы.
- Create `frontend/src/components/whiteboard/boardTools.test.ts` — тест маппингов.
- Create `frontend/src/components/whiteboard/BoardUi.tsx` — док + поповер.
- Modify `frontend/src/components/whiteboard/TldrawCanvas.tsx` — `hideUi`, `<BoardUi>`, `insertImageFile`.
- Modify `frontend/src/components/call/PipCameras.tsx` — круглый PiP сверху-справа.

---

## Task 1: boardTools.ts (маппинги + тест)

**Files:**
- Create: `frontend/src/components/whiteboard/boardTools.ts`
- Test: `frontend/src/components/whiteboard/boardTools.test.ts`

**Interfaces:**
- Produces:
  - `UiTool = 'select'|'hand'|'pen'|'eraser'|'text'|'shape'|'sticky'|'image'`
  - `TOOL_IDS: UiTool[]`
  - `toEditorTool(t: UiTool): string | null` (image → null)
  - `COLOR_MAP: Record<string,string>` (hex → имя цвета tldraw)
  - `COLORS: string[]` (порядок hex палитры)
  - `SIZE_KEYS: ['S','M','L','XL']`, `SIZE_MAP: Record<string,string>`
  - `FONT_DEFS: {key:string;label:string;family:string}[]`, `FONT_MAP: Record<string,string>`
  - `showColor(t)`, `showThickness(t)`, `showFont(t)`, `hasSettings(t)` → boolean

- [ ] **Step 1: Написать тест `boardTools.test.ts`**

```ts
import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  toEditorTool, COLOR_MAP, SIZE_MAP, FONT_MAP,
  showColor, showThickness, showFont, hasSettings, TOOL_IDS,
} from './boardTools.ts'

test('toEditorTool маппит инструменты', () => {
  assert.equal(toEditorTool('pen'), 'draw')
  assert.equal(toEditorTool('shape'), 'geo')
  assert.equal(toEditorTool('sticky'), 'note')
  assert.equal(toEditorTool('select'), 'select')
  assert.equal(toEditorTool('image'), null)
})

test('COLOR_MAP — hex мокапа в имена tldraw', () => {
  assert.equal(COLOR_MAP['#26262a'], 'black')
  assert.equal(COLOR_MAP['#e0564f'], 'red')
  assert.equal(COLOR_MAP['#4f7bd0'], 'blue')
})

test('SIZE_MAP и FONT_MAP', () => {
  assert.equal(SIZE_MAP['M'], 'm')
  assert.equal(SIZE_MAP['XL'], 'xl')
  assert.equal(FONT_MAP['hand'], 'draw')
  assert.equal(FONT_MAP['serif'], 'serif')
})

test('видимость секций поповера', () => {
  assert.equal(showColor('pen'), true)
  assert.equal(showColor('eraser'), false)
  assert.equal(showThickness('eraser'), true)
  assert.equal(showFont('text'), true)
  assert.equal(hasSettings('hand'), false)
  assert.equal(hasSettings('pen'), true)
})

test('TOOL_IDS — 8 инструментов в порядке мокапа', () => {
  assert.deepEqual(TOOL_IDS, ['select','hand','pen','eraser','text','shape','sticky','image'])
})
```

- [ ] **Step 2: Запустить тест — падает**

Run: `cd frontend && node --test --experimental-strip-types src/components/whiteboard/boardTools.test.ts`
Expected: FAIL (модуль не найден).

- [ ] **Step 3: Реализовать `boardTools.ts`**

```ts
export type UiTool =
  | 'select' | 'hand' | 'pen' | 'eraser' | 'text' | 'shape' | 'sticky' | 'image'

export const TOOL_IDS: UiTool[] = [
  'select', 'hand', 'pen', 'eraser', 'text', 'shape', 'sticky', 'image',
]

const EDITOR_TOOL: Record<UiTool, string | null> = {
  select: 'select', hand: 'hand', pen: 'draw', eraser: 'eraser',
  text: 'text', shape: 'geo', sticky: 'note', image: null,
}
export function toEditorTool(t: UiTool): string | null {
  return EDITOR_TOOL[t]
}

// hex мокапа → имя цвета tldraw (DefaultColorStyle)
export const COLORS = ['#26262a', '#e0564f', '#e6a43c', '#4f9d6e', '#4f7bd0', '#9168d6']
export const COLOR_MAP: Record<string, string> = {
  '#26262a': 'black', '#e0564f': 'red', '#e6a43c': 'orange',
  '#4f9d6e': 'green', '#4f7bd0': 'blue', '#9168d6': 'violet',
}

export const SIZE_KEYS = ['S', 'M', 'L', 'XL'] as const
export const SIZE_MAP: Record<string, string> = { S: 's', M: 'm', L: 'l', XL: 'xl' }
// диаметр точки-превью толщины (px), из мокапа
export const SIZE_DOT: Record<string, number> = { S: 4, M: 7, L: 11, XL: 15 }

export const FONT_DEFS = [
  { key: 'hand', label: 'Аа', family: "'Marck Script', cursive" },
  { key: 'sans', label: 'Аа', family: 'Inter, sans-serif' },
  { key: 'serif', label: 'Аа', family: 'Georgia, serif' },
]
export const FONT_MAP: Record<string, string> = { hand: 'draw', sans: 'sans', serif: 'serif' }

export function showColor(t: UiTool | null): boolean {
  return t === 'pen' || t === 'text' || t === 'shape' || t === 'sticky'
}
export function showThickness(t: UiTool | null): boolean {
  return t === 'pen' || t === 'eraser' || t === 'shape'
}
export function showFont(t: UiTool | null): boolean {
  return t === 'text'
}
export function hasSettings(t: UiTool | null): boolean {
  return !!t && (showColor(t) || showThickness(t) || showFont(t) || t === 'eraser')
}
```

- [ ] **Step 4: Запустить тест — проходит**

Run: `cd frontend && node --test --experimental-strip-types src/components/whiteboard/boardTools.test.ts`
Expected: PASS (5 тестов).

- [ ] **Step 5: Коммит**

```bash
git add frontend/src/components/whiteboard/boardTools.ts frontend/src/components/whiteboard/boardTools.test.ts
git commit -m "feat(board): tool/style mapping module + tests"
```

---

## Task 2: PipCameras — круглый PiP сверху-справа

**Files:**
- Modify: `frontend/src/components/call/PipCameras.tsx`

**Interfaces:**
- Consumes: ничего из других задач.
- Produces: ничего (визуальный компонент).

Дедуп-логика треков сохраняется как есть. Меняется только разметка/стили обёртки и тайла.

- [ ] **Step 1: Переписать разметку на круглый вид**

Заменить `return (...)` на:

```tsx
  return (
    <div
      style={{
        position: 'fixed',
        top: 72, // под доком
        right: 16,
        zIndex: 9999,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        gap: 10,
        pointerEvents: 'auto',
      }}
    >
      {uniqueTracks.map((track) => (
        <div
          key={`${track.participant.identity}-${track.source}`}
          style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 8 }}
        >
          <div
            style={{
              width: 86,
              height: 86,
              borderRadius: '50%',
              overflow: 'hidden',
              border: '3px solid rgba(255,255,255,.7)',
              boxShadow: '0 10px 28px rgba(30,40,60,.22)',
              background: 'linear-gradient(150deg,#3c3c44,#1f1f24)',
            }}
          >
            <ParticipantTile
              trackRef={track}
              style={{ width: '100%', height: '100%', borderRadius: 0 }}
            />
          </div>
          <div
            style={{
              display: 'flex', alignItems: 'center', gap: 6,
              background: 'rgba(255,255,255,.8)', padding: '4px 11px',
              borderRadius: 999, boxShadow: '0 4px 14px rgba(30,40,60,.12)',
            }}
          >
            <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="#2f333b" strokeWidth="2.1" strokeLinecap="round">
              <path d="M12 2a3 3 0 0 0-3 3v6a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3z" />
              <path d="M5 11a7 7 0 0 0 14 0M12 18v3" />
            </svg>
            <span style={{ color: '#2f333b', fontSize: 11.5, fontWeight: 600 }}>Репетитор</span>
          </div>
        </div>
      ))}
    </div>
  )
```

- [ ] **Step 2: Проверка сборкой**

Run: `cd frontend && npx tsc --noEmit -p tsconfig.json 2>&1 | grep PipCameras || echo "PipCameras OK"`
Expected: `PipCameras OK`.

- [ ] **Step 3: Коммит**

```bash
git add frontend/src/components/call/PipCameras.tsx
git commit -m "feat(board): round PiP with live video, top-right"
```

---

## Task 3: BoardUi + проводка в TldrawCanvas

**Files:**
- Create: `frontend/src/components/whiteboard/BoardUi.tsx`
- Modify: `frontend/src/components/whiteboard/TldrawCanvas.tsx`

**Interfaces:**
- Consumes: всё из `boardTools.ts` (Task 1).
- BoardUi props: `{ onInsertImage: () => void }`.

Заметки по API tldraw (проверено в node_modules v5.1.0):
- Активный тул: `editor.getCurrentToolId()`; смена: `editor.setCurrentTool(id)`.
- Стиль применять: `editor.setStyleForNextShapes(DefaultColorStyle, value)` **и**
  `editor.setStyleForSelectedShapes(DefaultColorStyle, value)`.
- Текущее значение (подсветка): `editor.getSharedStyles().getAsKnownValue(DefaultColorStyle)`.
- Undo/redo: `editor.undo()/redo()`, доступность `editor.getCanUndo()/getCanRedo()`.
- Удаление: `editor.deleteShapes(editor.getSelectedShapeIds())`.
- `track()` из `@tldraw/tldraw` делает компонент реактивным ко всем этим геттерам.

- [ ] **Step 1: Создать `BoardUi.tsx`**

```tsx
'use client'

import { useState } from 'react'
import {
  track, useEditor,
  DefaultColorStyle, DefaultSizeStyle, DefaultFontStyle,
} from '@tldraw/tldraw'
import {
  TOOL_IDS, COLORS, COLOR_MAP, SIZE_KEYS, SIZE_MAP, SIZE_DOT,
  FONT_DEFS, FONT_MAP, toEditorTool, showColor, showThickness, showFont, hasSettings,
  type UiTool,
} from './boardTools'
import { BoardPageMenu } from './BoardPageMenu'

// Тема paper (константы из мокапа Board.dc.html)
const T = {
  dockInnerBg: '#ffffff', dockRadius: 14, btnSize: 42, btnRadius: 11,
  activeBg: '#26262a', activeFg: '#ffffff', idleFg: '#4a4a50',
  accent: '#26262a', chipActive: '#efece4', textMut: '#8c8c93',
  popBg: '#ffffff', popRadius: 16, dividerColor: 'rgba(0,0,0,.08)',
}

const ICON: Record<UiTool, React.ReactNode> = {
  select: <path d="M3 3l7.07 16.97 2.51-7.39 7.39-2.51z" />,
  hand: <path d="M18 11V6a2 2 0 0 0-4 0M14 10V4a2 2 0 0 0-4 0v2M10 10.5V6a2 2 0 0 0-4 0v8M18 8a2 2 0 1 1 4 0v6a8 8 0 0 1-8 8h-2c-2.8 0-4.5-.86-5.99-2.34l-3.6-3.6a2 2 0 0 1 2.83-2.82L7 15" />,
  pen: <><path d="M17 3a2.83 2.83 0 0 1 4 4L7.5 20.5 2 22l1.5-5.5z" /><path d="M15 5l4 4" /></>,
  eraser: <><path d="M7 21l-4.3-4.3a1 1 0 0 1 0-1.4l10-10a1 1 0 0 1 1.4 0l5.6 5.6a1 1 0 0 1 0 1.4L13 21" /><path d="M22 21H7M5 11l9 9" /></>,
  text: <path d="M4 6.5V4.5h16v2M9 19.5h6M12 4.5v15" />,
  shape: <rect x="4" y="4" width="16" height="16" rx="3" />,
  sticky: <><path d="M15.5 3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h9l6-6V5a2 2 0 0 0-2-2z" /><path d="M14 21v-5a2 2 0 0 1 2-2h5" /></>,
  image: <><rect x="3" y="3" width="18" height="18" rx="3" /><circle cx="8.5" cy="9" r="1.6" /><path d="M21 16l-5-5L5 21" /></>,
}
const TITLE: Record<UiTool, string> = {
  select: 'Выделение', hand: 'Рука', pen: 'Перо', eraser: 'Ластик',
  text: 'Текст', shape: 'Фигура', sticky: 'Стикер', image: 'Картинка',
}

interface Props {
  onInsertImage: () => void
}

export const BoardUi = track(function BoardUi({ onInsertImage }: Props) {
  const editor = useEditor()
  const [openTool, setOpenTool] = useState<UiTool | null>(null)
  const [pagesOpen, setPagesOpen] = useState(false)

  const active = editor.getCurrentToolId()
  const curColor = editor.getSharedStyles().getAsKnownValue(DefaultColorStyle)
  const curSize = editor.getSharedStyles().getAsKnownValue(DefaultSizeStyle)
  const curFont = editor.getSharedStyles().getAsKnownValue(DefaultFontStyle)
  const canUndo = editor.getCanUndo()
  const canRedo = editor.getCanRedo()
  const hasSel = editor.getSelectedShapeIds().length > 0

  function pick(t: UiTool) {
    if (t === 'image') { onInsertImage(); return }
    const eid = toEditorTool(t)
    if (eid) editor.setCurrentTool(eid)
    setOpenTool((prev) => (hasSettings(t) ? (prev === t ? null : t) : null))
  }
  function setColor(hex: string) {
    const v = COLOR_MAP[hex]
    editor.setStyleForNextShapes(DefaultColorStyle, v)
    editor.setStyleForSelectedShapes(DefaultColorStyle, v)
  }
  function setSize(key: string) {
    const v = SIZE_MAP[key]
    editor.setStyleForNextShapes(DefaultSizeStyle, v)
    editor.setStyleForSelectedShapes(DefaultSizeStyle, v)
  }
  function setFont(key: string) {
    const v = FONT_MAP[key]
    editor.setStyleForNextShapes(DefaultFontStyle, v)
    editor.setStyleForSelectedShapes(DefaultFontStyle, v)
  }

  const iconBtn = (onClick: () => void, disabled: boolean, path: React.ReactNode) => (
    <button
      onClick={onClick}
      disabled={disabled}
      style={{
        width: 36, height: 36, display: 'flex', alignItems: 'center', justifyContent: 'center',
        border: 'none', background: 'transparent', borderRadius: 10,
        cursor: disabled ? 'default' : 'pointer', color: T.idleFg, opacity: disabled ? 0.35 : 1,
      }}
    >
      <svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round">{path}</svg>
    </button>
  )

  return (
    <div style={{ position: 'absolute', inset: 0, zIndex: 300, pointerEvents: 'none', fontFamily: "-apple-system,'SF Pro Text',system-ui,'Segoe UI',sans-serif", userSelect: 'none' }}>
      {/* ── Верхний док ── */}
      <div style={{ position: 'absolute', top: 0, left: 0, right: 0, pointerEvents: 'none' }}>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr auto 1fr', alignItems: 'center', padding: '14px 20px', pointerEvents: 'none' }}>

          {/* Левая группа */}
          <div style={{ position: 'relative', display: 'flex', alignItems: 'center', gap: 4, justifySelf: 'start', pointerEvents: 'auto' }}>
            {iconBtn(() => setPagesOpen((v) => !v), false, <path d="M4 7h16M4 12h16M4 17h16" />)}
            <button onClick={() => setPagesOpen((v) => !v)} style={{ display: 'flex', alignItems: 'center', gap: 6, border: 'none', background: 'transparent', borderRadius: 11, cursor: 'pointer', color: T.accent, fontSize: 13, fontWeight: 600, padding: '7px 11px' }}>
              Урок
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M6 9l6 6 6-6" /></svg>
            </button>
            <div style={{ width: 1, height: 20, background: 'rgba(0,0,0,.10)', margin: '0 5px' }} />
            {iconBtn(() => editor.undo(), !canUndo, <><path d="M9 14l-4-4 4-4" /><path d="M5 10h11a4 4 0 0 1 0 8h-3" /></>)}
            {iconBtn(() => editor.redo(), !canRedo, <><path d="M15 14l4-4-4-4" /><path d="M19 10H8a4 4 0 0 0 0 8h3" /></>)}
            {pagesOpen && (
              <div style={{ position: 'absolute', top: 44, left: 0, pointerEvents: 'auto' }} onPointerDown={(e) => e.stopPropagation()}>
                <BoardPageMenu />
              </div>
            )}
          </div>

          {/* Центр: инструменты */}
          <div style={{ justifySelf: 'center', display: 'flex', alignItems: 'center', gap: 4, padding: 6, background: T.dockInnerBg, borderRadius: T.dockRadius, boxShadow: '0 4px 16px rgba(30,30,34,0.10)', pointerEvents: 'auto' }}>
            {TOOL_IDS.map((t) => {
              const isActive = t !== 'image' && toEditorTool(t) === active
              return (
                <button key={t} onClick={() => pick(t)} title={TITLE[t]}
                  style={{ width: T.btnSize, height: T.btnSize, display: 'flex', alignItems: 'center', justifyContent: 'center', border: 'none', cursor: 'pointer', transition: 'background .15s,color .15s', borderRadius: T.btnRadius, background: isActive ? T.activeBg : 'transparent', color: isActive ? T.activeFg : T.idleFg }}>
                  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">{ICON[t]}</svg>
                </button>
              )
            })}
          </div>

          {/* Правая группа */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 4, justifySelf: 'end', pointerEvents: 'auto' }}>
            {iconBtn(() => editor.deleteShapes(editor.getSelectedShapeIds()), !hasSel, <path d="M5 7h14M10 7V5h4v2M7 7l1 12h8l1-12" />)}
            {/* ponytail: ⋮ — заглушка, add when needed */}
            {iconBtn(() => {}, false, <><circle cx="12" cy="5" r="1.5" /><circle cx="12" cy="12" r="1.5" /><circle cx="12" cy="19" r="1.5" /></>)}
          </div>
        </div>

        {/* ── Поповер настроек ── */}
        {openTool && hasSettings(openTool) && (
          <div style={{ position: 'absolute', left: '50%', top: 'calc(100% + 10px)', transform: 'translateX(-50%)', display: 'flex', alignItems: 'center', gap: 12, background: T.popBg, border: '1px solid rgba(0,0,0,0.06)', boxShadow: '0 14px 36px rgba(40,36,28,0.18)', borderRadius: T.popRadius, padding: '9px 14px', pointerEvents: 'auto' }}>
            {showColor(openTool) && (
              <div style={{ display: 'flex', alignItems: 'center', gap: 7 }}>
                {COLORS.map((hex) => {
                  const sel = curColor === COLOR_MAP[hex]
                  return <button key={hex} onClick={() => setColor(hex)} style={{ width: 21, height: 21, borderRadius: '50%', border: 'none', cursor: 'pointer', background: hex, boxShadow: sel ? `0 0 0 2px ${T.popBg},0 0 0 4px ${T.accent}` : 'inset 0 0 0 1px rgba(0,0,0,0.10)' }} />
                })}
              </div>
            )}
            {showColor(openTool) && showThickness(openTool) && <div style={{ width: 1, height: 24, background: T.dividerColor }} />}
            {showThickness(openTool) && (
              <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                {SIZE_KEYS.map((k) => {
                  const sel = curSize === SIZE_MAP[k]
                  return (
                    <button key={k} onClick={() => setSize(k)} style={{ width: 30, height: 30, display: 'flex', alignItems: 'center', justifyContent: 'center', border: 'none', borderRadius: 9, cursor: 'pointer', background: sel ? T.chipActive : 'transparent' }}>
                      <span style={{ display: 'block', width: SIZE_DOT[k], height: SIZE_DOT[k], borderRadius: '50%', background: sel ? T.accent : '#a3a3a9' }} />
                    </button>
                  )
                })}
              </div>
            )}
            {showFont(openTool) && (
              <>
                <div style={{ width: 1, height: 24, background: T.dividerColor }} />
                <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                  {FONT_DEFS.map((f) => {
                    const sel = curFont === FONT_MAP[f.key]
                    return <button key={f.key} onClick={() => setFont(f.key)} style={{ minWidth: 36, height: 30, padding: '0 9px', display: 'flex', alignItems: 'center', justifyContent: 'center', border: 'none', borderRadius: 9, cursor: 'pointer', fontSize: 15, fontFamily: f.family, background: sel ? T.chipActive : 'transparent', color: sel ? T.accent : T.textMut }}>{f.label}</button>
                  })}
                </div>
                <div style={{ width: 1, height: 24, background: T.dividerColor }} />
                <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                  {SIZE_KEYS.map((k) => {
                    const sel = curSize === SIZE_MAP[k]
                    return <button key={k} onClick={() => setSize(k)} style={{ width: 32, height: 30, display: 'flex', alignItems: 'center', justifyContent: 'center', border: 'none', borderRadius: 9, cursor: 'pointer', fontSize: 11, fontWeight: 600, background: sel ? T.chipActive : 'transparent', color: sel ? T.accent : T.textMut }}>{k}</button>
                  })}
                </div>
              </>
            )}
          </div>
        )}
      </div>
    </div>
  )
})
```

- [ ] **Step 2: Проводка в `TldrawCanvas.tsx`**

Добавить `hideUi` на `<Tldraw>` и рендер `<BoardUi>`. Добавить `insertImageFile` + скрытый инпут.

Импорт вверху файла:
```tsx
import { BoardUi } from './BoardUi'
```

В компоненте, рядом с `pdfRef`, добавить реф на файл-инпут и функцию вставки картинки:
```tsx
  const imageInputRef = useRef<HTMLInputElement | null>(null)

  const insertImageFile = async (file: File) => {
    const editor = editorRef.current
    if (!editor) return
    const url = URL.createObjectURL(file)
    try {
      const dim = await new Promise<{ w: number; h: number }>((resolve, reject) => {
        const img = new Image()
        img.onload = () => resolve({ w: img.naturalWidth, h: img.naturalHeight })
        img.onerror = reject
        img.src = url
      })
      const result = await whiteboardApi.uploadAsset(boardId, file)
      const assetId = AssetRecordType.createId()
      editor.createAssets([{
        id: assetId, type: 'image', typeName: 'asset',
        props: { src: `${BASE_URL}${result.url}`, w: dim.w, h: dim.h, mimeType: file.type, name: file.name, isAnimated: false },
        meta: {},
      }])
      const c = editor.getViewportPageBounds().center
      editor.createShape({ type: 'image', x: c.x - dim.w / 2, y: c.y - dim.h / 2, props: { assetId, w: dim.w, h: dim.h } })
    } catch {
      toast.error('Не удалось вставить картинку')
    } finally {
      URL.revokeObjectURL(url)
    }
  }
```

Внутри враппера `<div className="relative w-full h-full" ...>`, после `<Tldraw ...>`, добавить:
```tsx
        {!isGuest && <BoardUi onInsertImage={() => imageInputRef.current?.click()} />}
        <input
          ref={imageInputRef}
          type="file"
          accept="image/*"
          style={{ display: 'none' }}
          onChange={(e) => {
            const f = e.target.files?.[0]
            if (f) void insertImageFile(f)
            e.target.value = ''
          }}
        />
```

На `<Tldraw>` добавить проп `hideUi`.

Примечание: гостю (`isGuest`) BoardUi не показываем — у гостя нет прав редактирования страниц/ассетов (`BoardPageMenu` и upload требуют tutor-скоуп). Гость видит доску без дока (как read-only). Если нужен просмотр-тулбар гостю — отдельная задача.

- [ ] **Step 3: Проверка типов и тестов**

Run: `cd frontend && npx tsc --noEmit && node --test --experimental-strip-types src/components/whiteboard/boardTools.test.ts`
Expected: без ошибок TS; тесты PASS.

- [ ] **Step 4: Коммит**

```bash
git add frontend/src/components/whiteboard/BoardUi.tsx frontend/src/components/whiteboard/TldrawCanvas.tsx
git commit -m "feat(board): custom top dock + settings popover over tldraw"
```

---

## Self-Review (заполнить при исполнении)

- Покрытие spec: маппинги (T1), видео (T2), док+поповер+hideUi+image (T3) — все разделы spec покрыты.
- Фон холста: подгонка темы tldraw под `#f4f1ea` — опционально; если визуально расходится с мокапом, добавить `colorScheme`/CSS-переменную в T3 Step 2. Пометка, не блокирует.
