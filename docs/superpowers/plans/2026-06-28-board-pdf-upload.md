# Board PDF Upload Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Позволить вставлять выбранные страницы PDF (учебника) на доску tldraw через перетаскивание, конвертируя их в PNG на клиенте.

**Architecture:** Перехватываем drop файлов через опцию tldraw `experimental__onDropOnCanvas` (вызывается ДО дефолтной обработки drop, возвращает boolean). PDF → `return true` (блокируем tldraw), читаем число страниц через pdfjs → диалог диапазона → рендерим выбранные страницы в PNG → заливаем существующим `whiteboardApi.uploadAsset` → вставляем встык по горизонтали от точки дропа. Не-PDF → `return false`, tldraw вставляет картинки как обычно. Бэкенд не меняется.

**Tech Stack:** Next.js, React, TypeScript, `@tldraw/tldraw`, `pdfjs-dist` (уже установлены). Тесты — встроенный `node:test` + нативный TS-strip Node 24 (без новых зависимостей).

## Global Constraints

- Новых npm-зависимостей не добавлять — `pdfjs-dist` уже в проекте.
- Бэкенд (`handlers/whiteboard.go`, `UploadAsset`) не трогать — принимает PNG как есть.
- `parseRange` держать в отдельном файле БЕЗ импорта `pdfjs-dist`, иначе `node:test` не запустит проверку (pdfjs тянет браузерный canvas).
- Все строки UI — на русском (соответствует кодовой базе).
- Рендер страниц: `scale: 1.5`, формат `image/png`.
- URL ассета строится как `${BASE_URL}${result.url}`, где `BASE_URL` экспортируется из `@/lib/api/whiteboard`.

---

### Task 1: Чистый парсер диапазона страниц

**Files:**
- Create: `frontend/src/lib/pdfRange.ts`
- Test: `frontend/src/lib/pdfRange.test.ts`

**Interfaces:**
- Produces: `parseRange(input: string, numPages: number): [number, number]` — возвращает `[from, to]`, 1-индексированные, гарантированно `1 <= from <= to <= numPages`.

- [ ] **Step 1: Написать падающий тест**

Create `frontend/src/lib/pdfRange.test.ts`:

```ts
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { parseRange } from './pdfRange.ts'

test('диапазон "5-8"', () => {
  assert.deepEqual(parseRange('5-8', 100), [5, 8])
})

test('одна страница "5"', () => {
  assert.deepEqual(parseRange('5', 100), [5, 5])
})

test('перевёрнутый "8-5" меняет местами', () => {
  assert.deepEqual(parseRange('8-5', 100), [5, 8])
})

test('ноль клампится к 1', () => {
  assert.deepEqual(parseRange('0', 100), [1, 1])
})

test('верх клампится к numPages', () => {
  assert.deepEqual(parseRange('3-999', 100), [3, 100])
})

test('пустая строка = весь документ', () => {
  assert.deepEqual(parseRange('', 100), [1, 100])
})

test('мусор = весь документ', () => {
  assert.deepEqual(parseRange('abc', 100), [1, 100])
})

test('пробелы игнорируются', () => {
  assert.deepEqual(parseRange(' 5 - 8 ', 100), [5, 8])
})
```

- [ ] **Step 2: Запустить тест — убедиться, что падает**

Run: `cd frontend && node --test src/lib/pdfRange.test.ts`
Expected: FAIL — `Cannot find module './pdfRange.ts'` или `parseRange is not a function`.

- [ ] **Step 3: Реализовать парсер**

Create `frontend/src/lib/pdfRange.ts`:

```ts
// Парсит пользовательский ввод диапазона страниц ("5", "5-8") в нормализованный
// [from, to]. Любой невалидный ввод трактуется как «весь документ».
// Держится отдельно от pdf.ts (без импорта pdfjs), чтобы node:test мог его гонять.
export function parseRange(input: string, numPages: number): [number, number] {
  const clamp = (n: number) => Math.min(Math.max(n, 1), numPages)

  const trimmed = input.trim()
  if (trimmed === '') return [1, numPages]

  const parts = trimmed.split('-').map((p) => p.trim())
  const nums = parts.map((p) => Number(p))
  if (nums.some((n) => !Number.isInteger(n))) return [1, numPages]

  let from = clamp(nums[0])
  let to = nums.length > 1 ? clamp(nums[1]) : from
  if (from > to) [from, to] = [to, from]
  return [from, to]
}
```

- [ ] **Step 4: Запустить тест — убедиться, что проходит**

Run: `cd frontend && node --test src/lib/pdfRange.test.ts`
Expected: PASS — 8 tests passed.

- [ ] **Step 5: Коммит**

```bash
git add frontend/src/lib/pdfRange.ts frontend/src/lib/pdfRange.test.ts
git commit -m "feat: add PDF page-range parser with tests"
```

---

### Task 2: Утилиты загрузки и рендера PDF

**Files:**
- Create: `frontend/src/lib/pdf.ts`

**Interfaces:**
- Consumes: ничего из других задач.
- Produces:
  - `loadPdf(file: File): Promise<PDFDocumentProxy>` — настраивает worker, возвращает документ (`.numPages`).
  - `renderPages(pdf: PDFDocumentProxy, from: number, to: number, onProgress?: (done: number, total: number) => void): Promise<RenderedPage[]>`
  - `type RenderedPage = { blob: Blob; width: number; height: number }`

- [ ] **Step 1: Реализовать утилиты**

Create `frontend/src/lib/pdf.ts`. Логика рендера переносится из удаляемого
`PdfUploadToolbar.tsx` (Task 4):

```ts
import * as pdfjs from 'pdfjs-dist'
import type { PDFDocumentProxy } from 'pdfjs-dist'

pdfjs.GlobalWorkerOptions.workerSrc = new URL(
  'pdfjs-dist/build/pdf.worker.min.mjs',
  import.meta.url
).toString()

export type RenderedPage = { blob: Blob; width: number; height: number }

export async function loadPdf(file: File): Promise<PDFDocumentProxy> {
  const arrayBuffer = await file.arrayBuffer()
  return pdfjs.getDocument({ data: arrayBuffer }).promise
}

// Рендерит страницы [from, to] (включительно, 1-индексированные) в PNG.
export async function renderPages(
  pdf: PDFDocumentProxy,
  from: number,
  to: number,
  onProgress?: (done: number, total: number) => void
): Promise<RenderedPage[]> {
  const total = to - from + 1
  const pages: RenderedPage[] = []
  for (let i = from; i <= to; i++) {
    const page = await pdf.getPage(i)
    const viewport = page.getViewport({ scale: 1.5 })
    const canvas = document.createElement('canvas')
    canvas.width = viewport.width
    canvas.height = viewport.height
    await page.render({ canvas, viewport }).promise
    const blob = await new Promise<Blob>((res) =>
      canvas.toBlob((b) => res(b!), 'image/png')
    )
    pages.push({ blob, width: viewport.width, height: viewport.height })
    onProgress?.(pages.length, total)
  }
  return pages
}
```

- [ ] **Step 2: Проверить, что собирается**

Run: `cd frontend && npx tsc --noEmit`
Expected: без ошибок по `src/lib/pdf.ts` (импорт `PDFDocumentProxy` резолвится из `pdfjs-dist`).

- [ ] **Step 3: Коммит**

```bash
git add frontend/src/lib/pdf.ts
git commit -m "feat: add PDF load/render helpers"
```

---

### Task 3: Диалог выбора диапазона страниц

**Files:**
- Create: `frontend/src/components/whiteboard/PdfRangeDialog.tsx`

**Interfaces:**
- Consumes: `parseRange` из `@/lib/pdfRange`; UI-компоненты из `@/components/ui/dialog`, `@/components/ui/input`, `@/components/ui/button`.
- Produces:
  - `PdfRangeDialog` — props:
    ```ts
    {
      open: boolean
      numPages: number
      progress: { done: number; total: number } | null
      onConfirm: (from: number, to: number) => void
      onCancel: () => void
    }
    ```

- [ ] **Step 1: Реализовать диалог**

Create `frontend/src/components/whiteboard/PdfRangeDialog.tsx`. Сверься с
`@/components/ui/dialog` (Base UI, `Dialog`/`DialogContent`/`DialogTitle`) и существующим
использованием в `src/components/lessons/LessonQuickDialog.tsx` на предмет паттерна:

```tsx
'use client'

import { useState } from 'react'
import { parseRange } from '@/lib/pdfRange'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'

interface Props {
  open: boolean
  numPages: number
  progress: { done: number; total: number } | null
  onConfirm: (from: number, to: number) => void
  onCancel: () => void
}

export function PdfRangeDialog({ open, numPages, progress, onConfirm, onCancel }: Props) {
  const [value, setValue] = useState('')

  const handleConfirm = () => {
    const [from, to] = parseRange(value, numPages)
    onConfirm(from, to)
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onCancel() }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Вставить страницы PDF</DialogTitle>
        </DialogHeader>
        {progress ? (
          <p className="text-sm text-muted-foreground">
            Конвертация: {progress.done} / {progress.total}…
          </p>
        ) : (
          <div className="flex flex-col gap-2">
            <p className="text-sm text-muted-foreground">Всего страниц: {numPages}</p>
            <Input
              autoFocus
              placeholder={`Например: 5-8 или 5 (пусто — все ${numPages})`}
              value={value}
              onChange={(e) => setValue(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') handleConfirm() }}
            />
          </div>
        )}
        <DialogFooter>
          <Button variant="outline" onClick={onCancel} disabled={!!progress}>
            Отмена
          </Button>
          <Button onClick={handleConfirm} disabled={!!progress}>
            Вставить
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
```

- [ ] **Step 2: Проверить экспорты UI-диалога**

Run: `cd frontend && grep -E "export (function|const) (Dialog|DialogContent|DialogHeader|DialogTitle|DialogFooter)" src/components/ui/dialog.tsx`
Expected: все пять имён присутствуют. Если `DialogHeader`/`DialogFooter` отсутствуют — заменить на `<div>` с тем же layout (`flex flex-col gap-1` / `flex justify-end gap-2`), не добавляя их в ui-kit.

- [ ] **Step 3: Проверить, что собирается**

Run: `cd frontend && npx tsc --noEmit`
Expected: без ошибок по `PdfRangeDialog.tsx`.

- [ ] **Step 4: Коммит**

```bash
git add frontend/src/components/whiteboard/PdfRangeDialog.tsx
git commit -m "feat: add PDF page-range dialog"
```

---

### Task 4: Перехват drop PDF через experimental__onDropOnCanvas + удаление старого тулбара

**Files:**
- Modify: `frontend/src/components/whiteboard/TldrawCanvas.tsx`
- Delete: `frontend/src/components/whiteboard/PdfUploadToolbar.tsx`

**Interfaces:**
- Consumes: `loadPdf`, `renderPages` из `@/lib/pdf`; `PdfRangeDialog` из `./PdfRangeDialog`; `whiteboardApi`, `BASE_URL` из `@/lib/api/whiteboard`.

**Контекст по tldraw 5.1.0 (проверено в node_modules):**
- При drop `useCanvasEvents.onDrop` сначала вызывает `editor.options.experimental__onDropOnCanvas({ event, point })`; если вернуть `true` — tldraw пропускает дефолтную обработку файлов.
- Сигнатура: `experimental__onDropOnCanvas?(o: { event: React.DragEvent<Element>; point: VecLike }): boolean` (синхронная). Передаётся через проп `<Tldraw options={{ ... }} />` (`options?: Partial<TldrawOptions>`).
- НЕ использовать `editor.putExternalContent({type:'files'})` для делегирования — `putExternalContent` просто вызывает `externalContentHandlers['files']`, то есть тот же путь → рекурсия. `experimental__onDropOnCanvas` решает это: `return false` сам отдаёт управление дефолту.
- `point` (`{x,y}` в координатах страницы) — точка дропа, используем как origin для размещения.

- [ ] **Step 1: Убедиться, что PdfUploadToolbar нигде не импортируется**

Run: `cd frontend && grep -rn "PdfUploadToolbar" src/`
Expected: совпадения только внутри самого `PdfUploadToolbar.tsx`. Если есть импорт где-то ещё — удалить его в том файле перед удалением компонента.

- [ ] **Step 2: Удалить старый компонент**

```bash
git rm frontend/src/components/whiteboard/PdfUploadToolbar.tsx
```

- [ ] **Step 3: Подключить перехват drop и состояние диалога**

Modify `frontend/src/components/whiteboard/TldrawCanvas.tsx`.

Добавить импорты вверху (рядом с существующими). `useEffect` оставить, только если он ещё
используется после удаления `wb:insert-image` (Step 4 ниже); `useMemo`/`useRef`/`useState` — нужны.

```tsx
import { useMemo, useRef, useState } from 'react'
import { Tldraw, type Editor, AssetRecordType, type TldrawOptions } from '@tldraw/tldraw'
import { toast } from 'sonner'
import { loadPdf, renderPages } from '@/lib/pdf'
import { PdfRangeDialog } from './PdfRangeDialog'
import { whiteboardApi, BASE_URL } from '@/lib/api/whiteboard'
import type { PDFDocumentProxy } from 'pdfjs-dist'
```

Внутри компонента `TldrawCanvas`, рядом с `editorRef`, добавить состояние диалога, ref на pdf
и точку дропа:

```tsx
const pdfRef = useRef<PDFDocumentProxy | null>(null)
const [pdfDialog, setPdfDialog] = useState<{ numPages: number; point: { x: number; y: number } } | null>(null)
const [pdfProgress, setPdfProgress] = useState<{ done: number; total: number } | null>(null)
```

Открытие диалога вынесено в ref-функцию, чтобы колбэк в `options` оставался стабильным
(не пересоздавал editor) и при этом не ловил устаревшее замыкание:

```tsx
const openPdfRef = useRef<(file: File, point: { x: number; y: number }) => void>(() => {})
openPdfRef.current = async (file, point) => {
  try {
    const pdf = await loadPdf(file)
    pdfRef.current = pdf
    setPdfDialog({ numPages: pdf.numPages, point })
  } catch {
    toast.error('Не удалось открыть PDF')
  }
}

// experimental__onDropOnCanvas вызывается ДО дефолтной обработки drop.
// PDF → return true (блокируем tldraw) + открываем диалог; иначе return false
// → tldraw вставляет картинки как обычно. Стабильно через useMemo([]).
// ponytail: опция помечена experimental__ в tldraw 5.1.0 — потолок известен;
// при смене версии/движка точка перехвата переедет в новый Canvas-компонент.
const pdfOptions = useMemo<Partial<TldrawOptions>>(
  () => ({
    experimental__onDropOnCanvas: ({ event, point }) => {
      const file = Array.from(event.dataTransfer?.files ?? []).find(
        (f) => f.type === 'application/pdf'
      )
      if (!file) return false
      openPdfRef.current(file, { x: point.x, y: point.y })
      return true
    },
  }),
  []
)
```

Передать `options` в `<Tldraw>` (рядом с существующими пропами `store`, `licenseKey` и т.д.),
`onMount` остаётся как есть (`editorRef.current = editor`):

```tsx
<Tldraw
  key={page?.id ?? 'empty'}
  store={store}
  options={pdfOptions}
  licenseKey={process.env.NEXT_PUBLIC_TLDRAW_LICENSE_KEY}
  onMount={(editor) => {
    editorRef.current = editor
  }}
  colorScheme="system"
/>
```

- [ ] **Step 4: Добавить обработчик подтверждения диапазона, рендер диалога, убрать старый useEffect**

Удалить осиротевший `useEffect` с обработчиком `window 'wb:insert-image'` (строки ~34-63 в
исходном файле) — его единственным источником был удалённый `PdfUploadToolbar`. СОХРАНИ импорт
`AssetRecordType` (нужен ниже). Если это был единственный `useEffect`, убери `useEffect` из
импорта `react` (в Step 3 он уже не добавлен).

В том же компоненте добавить обработчик подтверждения (вставка встык по горизонтали от точки
дропа `pdfDialog.point`):

```tsx
const handlePdfConfirm = async (from: number, to: number) => {
  const pdf = pdfRef.current
  const editor = editorRef.current
  const origin = pdfDialog?.point
  if (!pdf || !editor || !origin) return
  setPdfProgress({ done: 0, total: to - from + 1 })
  try {
    const pages = await renderPages(pdf, from, to, (done, total) =>
      setPdfProgress({ done, total })
    )
    let x = origin.x
    const y = origin.y
    for (let i = 0; i < pages.length; i++) {
      const p = pages[i]
      const pngFile = new File([p.blob], `page-${from + i}.png`, { type: 'image/png' })
      let result
      try {
        result = await whiteboardApi.uploadAsset(boardId, pngFile)
      } catch {
        toast.error(`Не удалось загрузить страницу ${from + i}`)
        break
      }
      const assetId = AssetRecordType.createId()
      editor.createAssets([
        {
          id: assetId,
          type: 'image',
          typeName: 'asset',
          props: {
            src: `${BASE_URL}${result.url}`,
            w: p.width,
            h: p.height,
            mimeType: 'image/png',
            name: `page-${from + i}`,
            isAnimated: false,
          },
          meta: {},
        },
      ])
      editor.createShape({ type: 'image', x, y, props: { assetId, w: p.width, h: p.height } })
      x += p.width // встык по горизонтали
    }
  } finally {
    pdfRef.current = null
    setPdfProgress(null)
    setPdfDialog(null)
  }
}
```

Отрисовать диалог внутри возвращаемого JSX, рядом с `<Tldraw …/>` (внутри обёрточного `div`):

```tsx
{pdfDialog && (
  <PdfRangeDialog
    open
    numPages={pdfDialog.numPages}
    progress={pdfProgress}
    onConfirm={handlePdfConfirm}
    onCancel={() => {
      pdfRef.current = null
      setPdfDialog(null)
    }}
  />
)}
```

- [ ] **Step 5: Проверить, что собирается**

Run: `cd frontend && npx tsc --noEmit`
Expected: без ошибок. Если падает на типе `TldrawOptions` — проверь экспорт:
`grep -n "TldrawOptions" node_modules/@tldraw/tldraw/dist-cjs/index.d.ts` (реэкспортируется из `@tldraw/editor`).

- [ ] **Step 6: Прогнать линтер**

Run: `cd frontend && npx eslint src/components/whiteboard/TldrawCanvas.tsx src/components/whiteboard/PdfRangeDialog.tsx src/lib/pdf.ts src/lib/pdfRange.ts`
Expected: без ошибок (предупреждения допустимы, если они есть и в остальном проекте).

- [ ] **Step 7: Коммит**

```bash
git add frontend/src/components/whiteboard/TldrawCanvas.tsx
git commit -m "feat: insert PDF pages onto board via drag-drop with range dialog"
```

---

### Task 5: Ручная проверка в браузере

**Files:** нет (verification only).

- [ ] **Step 1: Запустить фронт и бэк**

```bash
# терминал 1
cd /home/dragonbrn/tutorgo && go build -o ./tmp/main.exe . && ./tmp/main.exe
# терминал 2
cd /home/dragonbrn/tutorgo/frontend && npm run dev
```

- [ ] **Step 2: Проверить сценарии на доске курса**

Открыть доску курса и проверить:
1. Перетащить PNG/JPG на холст → вставляется как раньше (дефолт tldraw, без ошибки «filetype is not allowed»).
2. Перетащить многостраничный PDF → открывается диалог с числом страниц.
3. Ввести «2-3» → вставляются ровно 2 страницы, встык по горизонтали, без перекрытия.
4. Ввести одну страницу «1» → одна страница.
5. Пустой ввод → весь документ (для небольшого PDF).
6. Отмена в диалоге → ничего не вставляется.
7. Битый/не-PDF, переименованный в .pdf → toast «Не удалось открыть PDF».

Expected: все 7 сценариев проходят; тулбар tldraw не исчезает (баг из `8fa2d23` не воспроизводится, т.к. кастомный тулбар удалён).

---

## Notes

- Бэкенд `UploadAsset` уже принимает PNG по `Content-Type` без валидации типа — изменений не требует.
- Размещение страниц «встык по горизонтали» от точки дропа: `x += width` после каждой; вертикаль фиксирована (`y = origin.y`).
- Тест `parseRange` гоняется встроенным раннером Node 24 (`node --test`), нативный TS-strip — без vitest/jest и без новых зависимостей. Это сознательно минимальный выбор для фронта, где тестового раннера нет.
