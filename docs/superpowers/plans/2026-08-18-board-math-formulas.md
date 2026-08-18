# Математические формулы на доске — план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Репетитор набирает формулу в Desmos-подобном поле, она появляется на доске картинкой, поверх которой можно рисовать, а ученик видит процесс набора в реальном времени.

**Architecture:** Формула — обычный `image`-элемент Excalidraw: `customData.formula.latex` источник истины, `fileId` — хеш от latex+цвет+кегль. По сети едет только LaTeX обычным диффом элемента; SVG каждый клиент рендерит у себя MathJax'ом и кладёт в свой `addFiles`. Ни бэкенда, ни S3, ни новых WS-сообщений.

**Tech Stack:** Next.js 16 / React 19 / TypeScript strict, Excalidraw 0.18.1, MathJax v4 (`@mathjax/src`) для рендера, MathLive 0.110 для ввода, тесты — `node --test`.

**Spec:** `docs/superpowers/specs/2026-08-18-board-math-formulas-design.md`

## Global Constraints

- Все команды выполняются из `frontend/`.
- Тест-раннера нет: только `node --test <file>.test.ts` (Node 24 TS-strip). Импорты в тестах — с расширением `.ts` (`allowImportingTsExtensions: true` уже в tsconfig).
- TypeScript strict. `any` не проходит ревью; для чужих нетипизированных мест — точечный `as` с комментарием.
- Комментарии и текст UI — по-русски, как во всём `frontend/src/components/whiteboard/`.
- MathJax импортируется **только** через `@mathjax/src/js/*` (экспорт-карта пакета сама резолвит в `mjs/`).
- MathLive импортируется **только** динамически (`await import('mathlive')`): при SSR condition `node` отдаёт сборку без `MathfieldElement`.
- В `customData` кладём **только строки**: `roundFloats` (`excalidrawSync.ts:33`) округляет любые числа в дереве элемента до 2 знаков.
- Ничего не добавляем в карту `files` (`registerFile`) — формулы в БД не персистятся как файлы.
- Go-код, миграции и `handlers/` в этом плане не трогаются вообще.

---

### Task 1: Рендер LaTeX → самодостаточный SVG

**Files:**
- Create: `frontend/src/lib/latexToSvg.ts`
- Create: `frontend/src/lib/latexToSvg.test.ts`
- Modify: `frontend/package.json` (зависимость `@mathjax/src`)

**Interfaces:**
- Consumes: ничего.
- Produces:
  - `type LatexSvg = { svg: string; width: number; height: number; error?: string }`
  - `latexToSvg(latex: string, opts?: { fontSize?: number; color?: string }): Promise<LatexSvg>`
  - `svgToDataUrl(svg: string): string`

- [ ] **Step 1: Поставить MathJax**

```bash
cd frontend && npm i @mathjax/src@4.1.3
```

- [ ] **Step 2: Написать падающий тест**

Create `frontend/src/lib/latexToSvg.test.ts`:

```ts
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { latexToSvg, svgToDataUrl } from './latexToSvg.ts'

const QUADRATIC = 'x=\\frac{-b\\pm\\sqrt{b^2-4ac}}{2a}'

test('рендерит самодостаточный SVG с px-размерами и заданным цветом', async () => {
  const r = await latexToSvg(QUADRATIC, { fontSize: 20, color: '#1971c2' })

  assert.equal(r.error, undefined)
  assert.ok(r.svg.includes('xmlns="http://www.w3.org/2000/svg"'))
  // currentColor внутри <img> не резолвится — всё стало бы чёрным.
  assert.ok(!r.svg.includes('currentColor'))
  assert.ok(r.svg.includes('#1971c2'))
  // width/height обязаны быть в px: единица ex в standalone SVG резолвится
  // от системного шрифта, а не от нашего.
  assert.match(r.svg, /width="\d+(\.\d+)?"/)
  assert.ok(!/width="[\d.]+ex"/.test(r.svg))
  assert.ok(r.width > 150 && r.width < 220, `ширина ${r.width}`)
  assert.ok(r.height > 40 && r.height < 55, `высота ${r.height}`)
})

test('размер линеен по кеглю', async () => {
  const big = await latexToSvg(QUADRATIC, { fontSize: 20 })
  const small = await latexToSvg(QUADRATIC, { fontSize: 10 })
  assert.ok(Math.abs(small.width - big.width / 2) < 0.05)
  assert.ok(Math.abs(small.height - big.height / 2) < 0.05)
})

test('кириллица рендерится глифами, а не системным шрифтом', async () => {
  const r = await latexToSvg('\\text{Скорость } v=\\frac{s}{t}')
  assert.equal(r.error, undefined)
  // <text> означает, что MathJax сдался и зовёт шрифт зрителя.
  assert.ok(!r.svg.includes('<text'), 'SVG перестал быть самодостаточным')
})

test('битый LaTeX не кидает, а возвращает error', async () => {
  const r = await latexToSvg('\\frac{1}{')
  assert.equal(r.error, 'Missing close brace')
})

test('dataURL — base64 и переживает кириллицу', async () => {
  const r = await latexToSvg('\\text{Масса}')
  const url = svgToDataUrl(r.svg)
  assert.ok(url.startsWith('data:image/svg+xml;base64,'))
  const back = Buffer.from(url.split(',')[1], 'base64').toString('utf8')
  assert.ok(back.includes('svg'))
})
```

- [ ] **Step 3: Запустить тест, убедиться что падает**

Run: `cd frontend && node --test src/lib/latexToSvg.test.ts`
Expected: FAIL — `Cannot find module './latexToSvg.ts'`

- [ ] **Step 4: Реализовать модуль**

Create `frontend/src/lib/latexToSvg.ts`:

```ts
// LaTeX → самодостаточный SVG (глифы путями, ноль внешних ссылок).
//
// liteAdaptor не трогает DOM, поэтому один и тот же модуль работает в браузере
// и под `node --test`. Импортировать этот файл дорого (~490 КБ gzip), поэтому
// вызывающий код грузит его через `await import()`.
import { mathjax } from '@mathjax/src/js/mathjax.js'
import { TeX } from '@mathjax/src/js/input/tex.js'
import { SVG } from '@mathjax/src/js/output/svg.js'
import { liteAdaptor } from '@mathjax/src/js/adaptors/liteAdaptor.js'
import { RegisterHTMLHandler } from '@mathjax/src/js/handlers/html.js'

import '@mathjax/src/js/input/tex/base/BaseConfiguration.js'
import '@mathjax/src/js/input/tex/ams/AmsConfiguration.js'
import '@mathjax/src/js/input/tex/newcommand/NewcommandConfiguration.js'
import '@mathjax/src/js/input/tex/noundefined/NoUndefinedConfiguration.js'
import '@mathjax/src/js/input/tex/color/ColorConfiguration.js'
import '@mathjax/src/js/input/tex/mathtools/MathtoolsConfiguration.js'
import '@mathjax/src/js/input/tex/physics/PhysicsConfiguration.js'
import '@mathjax/src/js/input/tex/cancel/CancelConfiguration.js'
import '@mathjax/src/js/input/tex/boldsymbol/BoldsymbolConfiguration.js'
import '@mathjax/src/js/input/tex/braket/BraketConfiguration.js'
import '@mathjax/src/js/input/tex/cases/CasesConfiguration.js'
import '@mathjax/src/js/input/tex/enclose/EncloseConfiguration.js'
import '@mathjax/src/js/input/tex/textmacros/TextMacrosConfiguration.js'
import '@mathjax/src/js/input/tex/upgreek/UpgreekConfiguration.js'
import '@mathjax/src/js/input/tex/unicode/UnicodeConfiguration.js'
import '@mathjax/src/js/input/tex/configmacros/ConfigMacrosConfiguration.js'
import '@mathjax/src/js/input/tex/mhchem/MhchemConfiguration.js'

// Урезать набор бессмысленно: 73% веса — глифы шрифта, все пакеты против
// одного base стоят +8.5% gzip. Поэтому берём функционально полный набор.
const PACKAGES = [
  'base', 'ams', 'newcommand', 'noundefined', 'color', 'mathtools', 'physics',
  'cancel', 'boldsymbol', 'braket', 'cases', 'enclose', 'textmacros',
  'upgreek', 'unicode', 'configmacros', 'mhchem',
]

const adaptor = liteAdaptor()
RegisterHTMLHandler(adaptor)

// Глифы за пределами базовой латиницы (кириллица, фрактура, скрипт) лежат в
// динамических чанках шрифта. MathJax просит их по имени модуля и до загрузки
// БРОСАЕТ retry-исключение — поэтому весь рендер идёт через handleRetriesFor.
// Шаблонная строка (а не конкатенация произвольного имени) нужна бандлеру:
// так Turbopack видит папку и нарезает чанки статически.
mathjax.asyncLoad = (name: string) => {
  const chunk = /dynamic\/([\w-]+)\.js$/.exec(name)?.[1]
  if (!chunk) return Promise.reject(new Error(`mathjax: неизвестный чанк ${name}`))
  return import(`@mathjax/mathjax-newcm-font/js/svg/dynamic/${chunk}.js`)
}

const doc = mathjax.document('', {
  InputJax: new TeX({ packages: PACKAGES }),
  // local — глифы уезжают в <defs> ЭТОГО же SVG. global вынес бы их в общий
  // documentwide defs, и картинка перестала бы быть самодостаточной.
  OutputJax: new SVG({ fontCache: 'local' }),
})

export type LatexSvg = {
  svg: string
  /** px при переданном кегле */
  width: number
  height: number
  /** текст ошибки MathJax; при наличии SVG вставлять на доску НЕЛЬЗЯ */
  error?: string
}

export async function latexToSvg(
  latex: string,
  { fontSize = 20, color = '#1e1e1e' }: { fontSize?: number; color?: string } = {}
): Promise<LatexSvg> {
  const raw: string = await mathjax.handleRetriesFor(() =>
    adaptor.innerHTML(doc.convert(latex, { display: true }) as never)
  )

  // Размер берём из viewBox (единица = 1/1000 em) — это единственный надёжный
  // источник. Атрибуты width/height приходят в ex, а 1ex шрифта = 0.442em, и
  // внутри <img> браузер посчитал бы ex от системного шрифта (ошибка ~13%).
  // options.exFactor (0.5) для этого тоже не годится — он про вёрстку страницы.
  const viewBox = /viewBox="([^"]*)"/.exec(raw)?.[1]
  if (!viewBox) return { svg: '', width: 0, height: 0, error: 'нет viewBox' }
  const [, , vbW, vbH] = viewBox.trim().split(/\s+/).map(Number)

  const width = +((vbW / 1000) * fontSize).toFixed(2)
  const height = +((vbH / 1000) * fontSize).toFixed(2)

  const svg = raw
    .replace(/width="[\d.]+ex"/, `width="${width}"`)
    .replace(/height="[\d.]+ex"/, `height="${height}"`)
    .replace(/ style="vertical-align:[^"]*"/, '')
    // Внутри <img> currentColor не резолвится и всё стало бы чёрным.
    // Подстановок ровно две, на узле-обёртке; глифы наследуют от него.
    .replaceAll('currentColor', color)

  // MathJax не кидает на битом вводе, а рисует merror-блок. Такой SVG зовёт
  // системный serif, то есть НЕ самодостаточен — на доску его не пускаем.
  const error = /data-mjx-error="([^"]*)"/.exec(svg)?.[1]

  return { svg, width, height, error }
}

/** base64 через UTF-8: голый btoa() падает на кириллице (InvalidCharacterError). */
export function svgToDataUrl(svg: string): string {
  const bytes = new TextEncoder().encode(svg)
  let bin = ''
  for (const b of bytes) bin += String.fromCharCode(b)
  return `data:image/svg+xml;base64,${btoa(bin)}`
}
```

- [ ] **Step 5: Запустить тест, убедиться что проходит**

Run: `cd frontend && node --test src/lib/latexToSvg.test.ts`
Expected: PASS, 5 тестов.

Если падает `btoa is not defined` — Node 24 его имеет глобально; проверить версию `node -v`.

- [ ] **Step 6: Коммит**

```bash
cd frontend && git add package.json package-lock.json src/lib/latexToSvg.ts src/lib/latexToSvg.test.ts
git commit -m "feat(board): рендер LaTeX в самодостаточный SVG"
```

---

### Task 2: Модель формулы — customData, fileId, гейт конвертации

**Files:**
- Create: `frontend/src/components/whiteboard/mathFormula.ts`
- Create: `frontend/src/components/whiteboard/mathFormula.test.ts`

**Interfaces:**
- Consumes: ничего (чистые функции).
- Produces:
  - `type FormulaData = { latex: string; v: '1' }`
  - `readFormula(el: { customData?: Record<string, unknown> } | null | undefined): FormulaData | null`
  - `formulaCustomData(latex: string): { formula: FormulaData }`
  - `fileIdForLatex(latex: string, color: string, fontSize: number): string`
  - `looksLikeMath(text: string): boolean`

- [ ] **Step 1: Написать падающий тест**

Create `frontend/src/components/whiteboard/mathFormula.test.ts`:

```ts
import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  readFormula,
  formulaCustomData,
  fileIdForLatex,
  looksLikeMath,
} from './mathFormula.ts'

test('fileId детерминирован и меняется от каждого входа', () => {
  const a = fileIdForLatex('x^2', '#1e1e1e', 20)
  assert.equal(a, fileIdForLatex('x^2', '#1e1e1e', 20))
  // Цвет и кегль запечены в SVG: без них addFiles пропустил бы новый файл
  // («file data is not updated») и картинка не перерисовалась бы.
  assert.notEqual(a, fileIdForLatex('x^3', '#1e1e1e', 20))
  assert.notEqual(a, fileIdForLatex('x^2', '#e03131', 20))
  assert.notEqual(a, fileIdForLatex('x^2', '#1e1e1e', 28))
})

test('fileId — короткая hex-строка', () => {
  assert.match(fileIdForLatex('x^2', '#1e1e1e', 20), /^math-[0-9a-f]{16}$/)
})

test('readFormula достаёт формулу и отсеивает чужие элементы', () => {
  assert.deepEqual(readFormula({ customData: formulaCustomData('E=mc^2') }), {
    latex: 'E=mc^2',
    v: '1',
  })
  assert.equal(readFormula({ customData: { generationData: {} } }), null)
  assert.equal(readFormula({}), null)
  assert.equal(readFormula(null), null)
  // мусор из будущего формата не должен ронять чтение
  assert.equal(readFormula({ customData: { formula: { latex: 42 } } }), null)
})

test('гейт пропускает математику и отсекает текст', () => {
  assert.ok(looksLikeMath('x^2+1'))
  assert.ok(looksLikeMath('1/2'))
  assert.ok(looksLikeMath('E=mc^2'))
  assert.ok(looksLikeMath('sqrt(2)'))

  // convertAsciiMathToLatex уничтожает пробелы: «Задача 5» превратилась бы
  // в произведение шести курсивных переменных.
  assert.equal(looksLikeMath('Задача 5'), false)
  assert.equal(looksLikeMath('Hello world'), false)
  assert.equal(looksLikeMath('Найти скорость'), false)
  assert.equal(looksLikeMath(''), false)
})
```

- [ ] **Step 2: Запустить тест, убедиться что падает**

Run: `cd frontend && node --test src/components/whiteboard/mathFormula.test.ts`
Expected: FAIL — модуль не найден.

- [ ] **Step 3: Реализовать модуль**

Create `frontend/src/components/whiteboard/mathFormula.ts`:

```ts
// Формула на доске — это image-элемент, у которого источник истины лежит в
// customData, а картинка каждый раз рендерится клиентом заново. Здесь — только
// чистые функции вокруг этого контракта, без React и без MathJax.

/** Версия формата: сменим схему — сможем отличить старые элементы. */
export type FormulaData = { latex: string; v: '1' }

export function formulaCustomData(latex: string): { formula: FormulaData } {
  return { formula: { latex, v: '1' } }
}

export function readFormula(
  el: { customData?: Record<string, unknown> } | null | undefined
): FormulaData | null {
  const raw = el?.customData?.formula as Partial<FormulaData> | undefined
  if (!raw || typeof raw.latex !== 'string') return null
  return { latex: raw.latex, v: '1' }
}

// FNV-1a, два прохода с разными смещениями → 64 бита. Хватает: коллизия
// означала бы показ чужой картинки, при сотнях формул на доске вероятность
// ничтожна, а crypto.subtle асинхронный и тут только мешал бы.
function fnv1a(input: string, seed: number): number {
  let h = seed
  for (let i = 0; i < input.length; i++) {
    h ^= input.charCodeAt(i)
    h = Math.imul(h, 0x01000193) >>> 0
  }
  return h >>> 0
}

/**
 * Ключ картинки. Цвет и кегль входят в хеш, потому что запечены в SVG:
 * Excalidraw пропускает addFiles для уже известного fileId, и без них смена
 * цвета формулы не перерисовала бы её.
 */
export function fileIdForLatex(latex: string, color: string, fontSize: number): string {
  const key = `${latex} ${color} ${fontSize}`
  const hi = fnv1a(key, 0x811c9dc5).toString(16).padStart(8, '0')
  const lo = fnv1a(key, 0x01000193).toString(16).padStart(8, '0')
  return `math-${hi}${lo}`
}

/**
 * Стоит ли прогонять выделенный текст через convertAsciiMathToLatex.
 *
 * Конвертер хорош на математике, но всегда съедает пробелы: «Задача 5»
 * становится «Задача5», то есть произведением курсивных переменных. Поэтому
 * кириллицу и фразы из слов не трогаем — открываем пустое поле.
 */
export function looksLikeMath(text: string): boolean {
  const s = text.trim()
  if (!s) return false
  if (/[Ѐ-ӿ]/.test(s)) return false
  if (/[A-Za-z]{2,}\s+[A-Za-z]{2,}/.test(s)) return false
  return /[0-9+\-*/^_=()]|sqrt|frac|pi|alpha|beta/.test(s)
}
```

- [ ] **Step 4: Запустить тест, убедиться что проходит**

Run: `cd frontend && node --test src/components/whiteboard/mathFormula.test.ts`
Expected: PASS, 4 теста.

- [ ] **Step 5: Коммит**

```bash
cd frontend && git add src/components/whiteboard/mathFormula.ts src/components/whiteboard/mathFormula.test.ts
git commit -m "feat(board): модель формулы — customData, fileId, гейт конвертации"
```

---

### Task 3: Локальный рендер формул сцены

**Files:**
- Create: `frontend/src/components/whiteboard/useMathFiles.ts`
- Create: `frontend/src/components/whiteboard/useMathFiles.test.ts`
- Modify: `frontend/src/components/whiteboard/ExcalidrawCanvas.tsx` (импорт хука + вызов в `onChange`, строки 541-548)

**Interfaces:**
- Consumes: `readFormula` (Task 2), `latexToSvg` / `svgToDataUrl` (Task 1).
- Produces:
  - `formulasNeedingRender(elements, known: Set<string>): { fileId: string; latex: string }[]`
  - `useMathFiles(apiRef: RefObject<ExcalidrawImperativeAPI | null>): { renderMissing: () => void }`

- [ ] **Step 1: Написать падающий тест на чистую часть**

Create `frontend/src/components/whiteboard/useMathFiles.test.ts`:

```ts
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { formulasNeedingRender } from './useMathFiles.ts'
import { formulaCustomData } from './mathFormula.ts'

const el = (over: Record<string, unknown>) => ({
  type: 'image',
  fileId: 'math-1',
  isDeleted: false,
  customData: formulaCustomData('x^2'),
  ...over,
})

test('возвращает формулы, которых нет в кэше', () => {
  const out = formulasNeedingRender([el({})] as never, new Set())
  assert.deepEqual(out, [{ fileId: 'math-1', latex: 'x^2' }])
})

test('молчит, когда всё уже отрендерено — иначе addFiles зациклит onChange', () => {
  const out = formulasNeedingRender([el({})] as never, new Set(['math-1']))
  assert.deepEqual(out, [])
})

test('игнорирует не-формулы и удалённые элементы', () => {
  const items = [
    el({ customData: undefined, fileId: 'photo-1' }),
    el({ isDeleted: true, fileId: 'math-2' }),
    { type: 'text', text: 'x^2', customData: formulaCustomData('x^2') },
  ]
  assert.deepEqual(formulasNeedingRender(items as never, new Set()), [])
})

test('дедуплицирует одинаковые формулы в одном проходе', () => {
  const items = [el({}), el({ id: 'b' })]
  assert.equal(formulasNeedingRender(items as never, new Set()).length, 1)
})
```

- [ ] **Step 2: Запустить тест, убедиться что падает**

Run: `cd frontend && node --test src/components/whiteboard/useMathFiles.test.ts`
Expected: FAIL — модуль не найден.

- [ ] **Step 3: Реализовать хук**

Create `frontend/src/components/whiteboard/useMathFiles.ts`:

```ts
'use client'

import { useCallback, useRef, type RefObject } from 'react'
import type { ExcalidrawImperativeAPI, BinaryFileData, DataURL } from '@excalidraw/excalidraw/types'
import type { ExcalidrawElement, FileId } from '@excalidraw/excalidraw/element/types'
import { readFormula } from './mathFormula'

/**
 * Формулы сцены, для которых картинки ещё нет.
 *
 * Отдельная чистая функция, потому что это единственная логика, которую можно
 * проверить тестом: остальное — вызовы Excalidraw.
 */
export function formulasNeedingRender(
  elements: readonly ExcalidrawElement[],
  known: ReadonlySet<string>
): { fileId: string; latex: string }[] {
  const out: { fileId: string; latex: string }[] = []
  const seen = new Set(known)
  for (const el of elements) {
    if (el.type !== 'image' || el.isDeleted || !el.fileId) continue
    const formula = readFormula(el)
    if (!formula || seen.has(el.fileId)) continue
    seen.add(el.fileId)
    out.push({ fileId: el.fileId, latex: formula.latex })
  }
  return out
}

/**
 * Держит картинки формул в актуальном состоянии: LaTeX едет по сети, SVG
 * рендерится локально у каждого участника.
 *
 * Вызывается из onChange, то есть на КАЖДЫЙ кадр панорамирования — поэтому
 * ранний выход обязан быть дешёвым. И главное: api.addFiles() безусловно зовёт
 * scene.triggerUpdate() (даже когда ничего не добавил), а тот вызывает onChange
 * снова. Без множества уже отрендеренного это бесконечный цикл.
 */
export function useMathFiles(apiRef: RefObject<ExcalidrawImperativeAPI | null>) {
  const doneRef = useRef(new Set<string>())
  const inFlightRef = useRef(false)

  const renderMissing = useCallback(() => {
    const api = apiRef.current
    if (!api || inFlightRef.current) return

    const pending = formulasNeedingRender(api.getSceneElements(), doneRef.current)
    if (pending.length === 0) return

    inFlightRef.current = true
    void (async () => {
      try {
        // Модуль тяжёлый (~490 КБ gzip) — грузим при первой формуле на странице.
        const { latexToSvg, svgToDataUrl } = await import('@/lib/latexToSvg')
        const files: BinaryFileData[] = []

        for (const { fileId, latex } of pending) {
          const { svg, error } = await latexToSvg(latex, { color: colorOf(api, fileId) })
          // Битый SVG не отдаём: Excalidraw пометил бы элемент status:'error'
          // через newElementWith, а это version++ — порча уехала бы всем пирам.
          if (error || !svg) {
            doneRef.current.add(fileId)
            continue
          }
          doneRef.current.add(fileId)
          files.push({
            id: fileId as FileId,
            dataURL: svgToDataUrl(svg) as DataURL,
            mimeType: 'image/svg+xml',
            created: Date.now(),
          })
        }

        if (files.length > 0) apiRef.current?.addFiles(files)
      } catch (err) {
        console.warn('Формулу не удалось отрендерить', err)
      } finally {
        inFlightRef.current = false
      }
    })()
  }, [apiRef])

  return { renderMissing }
}

/** Цвет запечён в fileId, но сам SVG красим по элементу — берём его цвет обводки. */
function colorOf(api: ExcalidrawImperativeAPI, fileId: string): string {
  const el = api.getSceneElements().find((e) => e.type === 'image' && e.fileId === fileId)
  return (el?.customData?.color as string) ?? '#1e1e1e'
}
```

- [ ] **Step 4: Запустить тест, убедиться что проходит**

Run: `cd frontend && node --test src/components/whiteboard/useMathFiles.test.ts`
Expected: PASS, 4 теста.

- [ ] **Step 5: Подключить хук в ExcalidrawCanvas**

В `frontend/src/components/whiteboard/ExcalidrawCanvas.tsx`:

Импорт рядом с остальными импортами доски:

```ts
import { useMathFiles } from './useMathFiles'
```

Внутри компонента, рядом с другими хуками:

```ts
const { renderMissing } = useMathFiles(apiRef)
```

В обработчике `onChange` (сейчас строки 541-548) добавить вызов первой строкой тела:

```tsx
onChange={(elements, appState) => {
  renderMissing()
  keepAspect(elements)
  fitOnFirstVisit(elements)
  syncFollowTarget(appState.userToFollow?.socketId ?? null)
  onChange()
}}
```

- [ ] **Step 6: Проверить сборку**

Run: `cd frontend && npx tsc --noEmit && npm run lint`
Expected: без ошибок.

- [ ] **Step 7: Коммит**

```bash
cd frontend && git add src/components/whiteboard/useMathFiles.ts src/components/whiteboard/useMathFiles.test.ts src/components/whiteboard/ExcalidrawCanvas.tsx
git commit -m "feat(board): локальный рендер формул из customData"
```

---

### Task 4: Редактор формул и вставка на доску

**Files:**
- Create: `frontend/src/components/whiteboard/MathEditor.tsx`
- Create: `frontend/src/types/mathlive.d.ts`
- Modify: `frontend/src/components/whiteboard/ExcalidrawCanvas.tsx` (состояние редактора, кнопка в `renderTopRightUI` — сейчас строки 567-592)
- Modify: `frontend/package.json` (зависимость `mathlive`)

**Interfaces:**
- Consumes: `fileIdForLatex`, `formulaCustomData`, `looksLikeMath` (Task 2); `latexToSvg` (Task 1).
- Produces:
  - `<MathEditor initialLatex={string} onDraft={(latex: string) => void} onCommit={(latex: string) => void} onCancel={() => void} />`
  - в `ExcalidrawCanvas`: `insertFormula(latex: string, pos: {x,y}): Promise<void>`, `updateFormula(elementId: string, latex: string): Promise<void>`

- [ ] **Step 1: Поставить MathLive и объявить типы**

```bash
cd frontend && npm i mathlive@0.110.0
```

Create `frontend/src/types/mathlive.d.ts`:

```ts
// Пакет не объявляет JSX-типов. Нам достаточно тега: сам элемент создаётся
// через new MathfieldElement() в эффекте, поэтому в JSX он не рендерится.
import type { MathfieldElement } from 'mathlive'

declare global {
  interface HTMLElementTagNameMap {
    'math-field': MathfieldElement
  }
}
```

- [ ] **Step 2: Реализовать редактор**

Create `frontend/src/components/whiteboard/MathEditor.tsx`:

```tsx
'use client'

import { useEffect, useRef } from 'react'
import 'mathlive/fonts.css'

// Живой набор: ученик видит формулу по мере ввода. Не троттл, а debounce с
// потолком — Excalidraw НИКОГДА не удаляет файлы из памяти, а каждый кадр
// набора порождает новый fileId. Посимвольная трансляция оставила бы сотню
// SVG на формулу у каждого участника.
const DRAFT_DEBOUNCE_MS = 250
const DRAFT_MAX_WAIT_MS = 1000

interface Props {
  initialLatex: string
  /** промежуточный кадр — уезжает пирам, в историю не пишется */
  onDraft: (latex: string) => void
  /** финал: Enter или кнопка «Готово» */
  onCommit: (latex: string) => void
  onCancel: () => void
  /** экранные координаты левого-нижнего угла формулы */
  anchor: { left: number; top: number }
}

export function MathEditor({ initialLatex, onDraft, onCommit, onCancel, anchor }: Props) {
  const hostRef = useRef<HTMLDivElement>(null)
  // Колбэки в ref: эффект монтирует поле один раз, пересоздавать его на каждый
  // ре-рендер родителя нельзя — потеряется каретка и фокус.
  const cbRef = useRef({ onDraft, onCommit, onCancel })
  cbRef.current = { onDraft, onCommit, onCancel }

  useEffect(() => {
    let disposed = false
    let timer: ReturnType<typeof setTimeout> | null = null
    let firstEditAt = 0
    let field: HTMLElement | null = null

    void (async () => {
      // Статический импорт нельзя: при SSR condition "node" отдаёт сборку без
      // MathfieldElement, и он молча оказывается undefined.
      const { MathfieldElement } = await import('mathlive')
      if (disposed || !hostRef.current) return

      // Ни одного сетевого запроса: шрифты уже пришли из fonts.css, звуки не нужны.
      MathfieldElement.fontsDirectory = null
      MathfieldElement.soundsDirectory = null

      const mf = new MathfieldElement()
      field = mf
      mf.value = initialLatex
      // На доске рядом стилус — автопоказ клавиатуры на touch мешал бы.
      mf.mathVirtualKeyboardPolicy = 'manual'
      mf.style.cssText = 'min-width:320px;font-size:20px;padding:6px 8px;border:none;outline:none;background:transparent;color:var(--foreground)'

      const flush = () => {
        if (timer) clearTimeout(timer)
        timer = null
        firstEditAt = 0
        cbRef.current.onDraft(mf.value)
      }

      mf.addEventListener('input', () => {
        if (!firstEditAt) firstEditAt = Date.now()
        if (Date.now() - firstEditAt >= DRAFT_MAX_WAIT_MS) {
          flush()
          return
        }
        if (timer) clearTimeout(timer)
        timer = setTimeout(flush, DRAFT_DEBOUNCE_MS)
      })

      mf.addEventListener('keydown', (e) => {
        if (e.key === 'Enter') {
          e.preventDefault()
          cbRef.current.onCommit(mf.value)
        }
        if (e.key === 'Escape') {
          e.preventDefault()
          cbRef.current.onCancel()
        }
      })

      hostRef.current.append(mf)
      mf.focus()
    })()

    return () => {
      disposed = true
      if (timer) clearTimeout(timer)
      field?.remove()
    }
  }, [initialLatex])

  return (
    <div
      data-board-ui
      style={{
        position: 'absolute',
        left: anchor.left,
        top: anchor.top,
        zIndex: 6,
        display: 'flex',
        alignItems: 'center',
        gap: 8,
        padding: 6,
        borderRadius: 10,
        border: '1px solid var(--border)',
        background: 'var(--card)',
        boxShadow: '0 8px 24px rgb(0 0 0 / 0.12)',
      }}
      // Клики по редактору не должны уходить в холст и снимать выделение.
      onPointerDown={(e) => e.stopPropagation()}
    >
      <div ref={hostRef} />
      <button
        type="button"
        onClick={() => window.mathVirtualKeyboard.show()}
        style={{ padding: '4px 8px', fontSize: 13, cursor: 'pointer' }}
      >
        Символы
      </button>
    </div>
  )
}
```

Добавить в `frontend/src/app/globals.css` (клавиатура MathLive иначе уедет под UI доски — у неё z-index 105, у Excalidraw до 999999):

```css
body > .ML__keyboard {
  --keyboard-zindex: 1000000;
}
```

- [ ] **Step 3: Подключить вставку формулы в ExcalidrawCanvas**

В `ExcalidrawCanvas.tsx` добавить состояние и функции (рядом с `pdfDialog`-состоянием):

```tsx
const [mathEditor, setMathEditor] = useState<{
  /** id уже стоящей на доске формулы; null — формула ещё не создана */
  elementId: string | null
  latex: string
  /** экранные координаты поля ввода */
  anchor: { left: number; top: number }
  /** куда и с какими свойствами ставить новую формулу */
  place: {
    x: number
    y: number
    angle: number
    groupIds: string[]
    frameId: string | null
  }
} | null>(null)
```

`place` обязателен: `convertToExcalidrawElements` не наследует `angle`,
`groupIds` и `frameId`, и без явного переноса формула вывалится из группы,
выпадет из фрейма и потеряет поворот исходного текста.

Вставка новой формулы и обновление существующей:

```tsx
// Кадр набора: рисуем формулу и мутируем элемент. Промежуточные кадры не
// попадают в историю — Ctrl+Z должен откатывать формулу целиком.
const applyFormula = useCallback(
  async (elementId: string | null, latex: string, commit: boolean) => {
    const api = apiRef.current
    if (!api || !latex.trim()) return

    const color = api.getAppState().currentItemStrokeColor
    const fontSize = api.getAppState().currentItemFontSize
    const { latexToSvg, svgToDataUrl } = await import('@/lib/latexToSvg')
    const { svg, width, height, error } = await latexToSvg(latex, { fontSize, color })
    if (error || !svg) return

    const fileId = fileIdForLatex(latex, color, fontSize) as FileId
    api.addFiles([
      {
        id: fileId,
        dataURL: svgToDataUrl(svg) as DataURL,
        mimeType: 'image/svg+xml',
        created: Date.now(),
      },
    ])

    const elements = api.getSceneElementsIncludingDeleted()
    const existing = elementId ? elements.find((e) => e.id === elementId) : null

    if (existing) {
      // Пользователь мог растянуть формулу руками — держим его ширину,
      // высоту пересчитываем по новому соотношению сторон.
      const nextWidth = existing.width
      const nextHeight = +(nextWidth * (height / width)).toFixed(2)
      api.updateScene({
        elements: elements.map((e) =>
          e.id === elementId
            ? newElementWith(e, {
                fileId,
                width: nextWidth,
                height: nextHeight,
                customData: { ...e.customData, ...formulaCustomData(latex), color },
              })
            : e
        ),
        captureUpdate: commit ? CaptureUpdateAction.IMMEDIATELY : CaptureUpdateAction.NEVER,
      })
      return
    }

    const place = mathEditor?.place
    if (!place) return
    const [el] = convertToExcalidrawElements([
      {
        type: 'image',
        fileId,
        x: place.x,
        y: place.y,
        width,
        height,
        angle: place.angle,
        groupIds: place.groupIds,
        frameId: place.frameId,
        customData: { ...formulaCustomData(latex), color },
      },
    ])
    api.updateScene({
      elements: [...elements, el],
      captureUpdate: CaptureUpdateAction.IMMEDIATELY,
    })
    setMathEditor((s) => (s ? { ...s, elementId: el.id } : s))
  },
  [mathEditor]
)
```

Открытие пустого редактора: формула ставится в центр видимой области, поле
ввода — под ней. `viewportCoordsToSceneCoords` уже импортирован в файле.

```tsx
const openNewFormula = useCallback(() => {
  const api = apiRef.current
  const rect = wrapRef.current?.getBoundingClientRect()
  if (!api || !rect) return
  const scene = viewportCoordsToSceneCoords(
    { clientX: rect.left + rect.width / 2, clientY: rect.top + rect.height / 2 },
    api.getAppState()
  )
  setMathEditor({
    elementId: null,
    latex: '',
    anchor: { left: rect.width / 2 - 180, top: rect.height - 180 },
    place: { x: scene.x, y: scene.y, angle: 0, groupIds: [], frameId: null },
  })
}, [])
```

Кнопку добавить в `renderTopRightUI` рядом с «Материалы» (стили скопировать с соседней кнопки, чтобы не плодить новый вид):

```tsx
<button data-board-ui title="Формула" onClick={openNewFormula} style={/* как у «Материалы» */}>
  <Sigma size={18} strokeWidth={1.75} />
  Формула
</button>
```

`Sigma` импортируется из `lucide-react` рядом с `AudioLines`.

Рендер редактора — рядом с `<MediaPlayer />`:

```tsx
{mathEditor && (
  <MathEditor
    initialLatex={mathEditor.latex}
    anchor={mathEditor.anchor}
    onDraft={(latex) => void applyFormula(mathEditor.elementId, latex, false)}
    onCommit={(latex) => {
      void applyFormula(mathEditor.elementId, latex, true)
      setMathEditor(null)
    }}
    onCancel={() => setMathEditor(null)}
  />
)}
```

- [ ] **Step 4: Проверить сборку и линт**

Run: `cd frontend && npx tsc --noEmit && npm run lint`
Expected: без ошибок.

- [ ] **Step 5: Ручная проверка в браузере**

Run: `cd frontend && npm run dev`, открыть доску урока.
Проверить: кнопка «Формула» открывает поле; `1/2` даёт дробь; `sqrt` — корень; формула появляется на доске по мере набора; Enter закрывает редактор; Esc отменяет; в консоли нет запросов к `unpkg`/CDN.

- [ ] **Step 6: Коммит**

```bash
cd frontend && git add package.json package-lock.json src/types/mathlive.d.ts src/components/whiteboard/MathEditor.tsx src/components/whiteboard/ExcalidrawCanvas.tsx src/app/globals.css
git commit -m "feat(board): редактор формул MathLive с живым набором"
```

---

### Task 5: Правка формулы даблкликом

**Files:**
- Modify: `frontend/src/components/whiteboard/ExcalidrawCanvas.tsx` (обёртка — сейчас строка 496, где уже висят `onDropCapture` и `onPasteCapture`)

**Interfaces:**
- Consumes: `readFormula` (Task 2), состояние `mathEditor` и `applyFormula` (Task 4).
- Produces: ничего нового.

- [ ] **Step 1: Добавить перехват даблклика**

В `ExcalidrawCanvas.tsx`, на обёртке рядом с существующими capture-обработчиками:

```tsx
onDoubleClickCapture={(e) => {
  const api = apiRef.current
  if (!api) return
  const selected = api.getAppState().selectedElementIds
  const el = api.getSceneElements().find((x) => selected[x.id])
  const formula = readFormula(el)
  // Без проверки на формулу мы отобрали бы у обычных картинок штатную
  // обрезку: даблклик по image в 0.18.1 включает crop-режим.
  if (!el || !formula) return
  e.preventDefault()
  e.stopPropagation()
  const rect = wrapRef.current?.getBoundingClientRect()
  setMathEditor({
    elementId: el.id,
    latex: formula.latex,
    anchor: { left: e.clientX - (rect?.left ?? 0) - 160, top: e.clientY - (rect?.top ?? 0) + 24 },
    // формула уже стоит на доске; place не используется, но держим тип целым
    place: { x: el.x, y: el.y, angle: el.angle, groupIds: [...el.groupIds], frameId: el.frameId },
  })
}}
```

- [ ] **Step 2: Проверить сборку**

Run: `cd frontend && npx tsc --noEmit && npm run lint`
Expected: без ошибок.

- [ ] **Step 3: Ручная проверка**

- даблклик по формуле открывает редактор с её текстом, правка видна на доске;
- даблклик по обычной картинке (вставить любую через Ctrl+V) по-прежнему включает обрезку;
- Ctrl+Z после правки откатывает формулу целиком, а не по одному символу.

- [ ] **Step 4: Коммит**

```bash
cd frontend && git add src/components/whiteboard/ExcalidrawCanvas.tsx
git commit -m "feat(board): правка формулы по даблклику"
```

---

### Task 6: Кнопка в панели свойств Excalidraw

**Files:**
- Create: `frontend/src/components/whiteboard/MathShapeAction.tsx`
- Create: `frontend/src/components/whiteboard/mathShapeAction.test.ts`
- Modify: `frontend/src/components/whiteboard/ExcalidrawCanvas.tsx` (рендер компонента внутри обёртки)

**Interfaces:**
- Consumes: `readFormula`, `looksLikeMath` (Task 2); колбэки из `ExcalidrawCanvas`.
- Produces: `<MathShapeAction selection={...} onConvert={...} onEdit={...} onToText={...} />`

- [ ] **Step 1: Написать тест-сторож на класс панели**

Create `frontend/src/components/whiteboard/mathShapeAction.test.ts`:

```ts
// Кнопка «В формулу» встраивается порталом в чужую панель свойств: публичного
// API для этого у Excalidraw нет. Апгрейд пакета, переименовавший класс,
// уронил бы кнопку молча — этот тест падает вместо неё.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readdir, readFile } from 'node:fs/promises'

const DIST = 'node_modules/@excalidraw/excalidraw/dist/prod'

test('панель свойств Excalidraw всё ещё зовётся .panelColumn', async () => {
  const files = (await readdir(DIST)).filter((f) => f.endsWith('.js'))
  const sources = await Promise.all(files.map((f) => readFile(`${DIST}/${f}`, 'utf8')))
  assert.ok(
    sources.some((s) => s.includes('panelColumn')),
    'класс panelColumn исчез — сверь MathShapeAction.tsx с новой вёрсткой панели'
  )
})
```

- [ ] **Step 2: Запустить тест**

Run: `cd frontend && node --test src/components/whiteboard/mathShapeAction.test.ts`
Expected: PASS (класс есть в 0.18.1).

- [ ] **Step 3: Реализовать портал**

Create `frontend/src/components/whiteboard/MathShapeAction.tsx`:

```tsx
'use client'

import { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'

/**
 * Кнопка внутри панели свойств Excalidraw.
 *
 * Официальной точки расширения нет: UIOptions знает только canvasActions и
 * tools.image, а панель зовёт renderAction по фиксированным именам. Поэтому —
 * портал в живой DOM.
 *
 * Цель — .panelColumn, а не .App-menu__left: первый существует и в мобильной
 * вёрстке. Контейнер размонтируется при снятии выделения, и React создаёт
 * НОВЫЙ узел, поэтому цель переопрашивается по сигналу извне (onChange доски)
 * плюс MutationObserver как подстраховка.
 */
interface Props {
  /** меняется при каждом onChange доски — повод переспросить контейнер */
  revision: number
  container: HTMLElement | null
  mode: 'text' | 'formula' | null
  onConvert: () => void
  onEdit: () => void
  onToText: () => void
}

export function MathShapeAction({ revision, container, mode, onConvert, onEdit, onToText }: Props) {
  const [target, setTarget] = useState<HTMLElement | null>(null)

  useEffect(() => {
    if (!container) return
    const find = () => setTarget(container.querySelector<HTMLElement>('.panelColumn'))
    find()
    const observer = new MutationObserver(find)
    observer.observe(container, { childList: true, subtree: true })
    return () => observer.disconnect()
  }, [container, revision])

  if (!target || !mode) return null

  return createPortal(
    <div style={{ order: -1, display: 'flex', gap: 6, paddingBottom: 4 }}>
      {mode === 'text' ? (
        <button type="button" onClick={onConvert} style={buttonStyle}>
          ∑ В формулу
        </button>
      ) : (
        <>
          <button type="button" onClick={onEdit} style={buttonStyle}>
            ∑ Изменить
          </button>
          <button type="button" onClick={onToText} style={buttonStyle}>
            В текст
          </button>
        </>
      )}
    </div>,
    target
  )
}

const buttonStyle: React.CSSProperties = {
  padding: '4px 8px',
  fontSize: 12,
  borderRadius: 6,
  border: '1px solid var(--border)',
  background: 'var(--card)',
  color: 'var(--foreground)',
  cursor: 'pointer',
}
```

- [ ] **Step 4: Подключить в ExcalidrawCanvas**

Добавить состояние выделения, обновляемое в `onChange`:

```tsx
const [selection, setSelection] = useState<{ mode: 'text' | 'formula' | null; id: string | null }>({
  mode: null,
  id: null,
})
const [panelRevision, setPanelRevision] = useState(0)
```

В `onChange` (после `renderMissing()`):

```tsx
const api = apiRef.current
if (api) {
  const ids = appState.selectedElementIds
  const picked = elements.filter((el) => ids[el.id] && !el.isDeleted)
  const one = picked.length === 1 ? picked[0] : null
  const next = one
    ? readFormula(one)
      ? ({ mode: 'formula', id: one.id } as const)
      : one.type === 'text'
        ? ({ mode: 'text', id: one.id } as const)
        : ({ mode: null, id: null } as const)
    : ({ mode: null, id: null } as const)
  setSelection((prev) => (prev.mode === next.mode && prev.id === next.id ? prev : next))
  setPanelRevision((n) => n + 1)
}
```

Замена текста формулой и обратно:

```tsx
// Excalidraw не даёт сменить type, поэтому переключение — это удаление
// старого элемента и вставка нового на его месте.
const convertTextToFormula = useCallback(async () => {
  const api = apiRef.current
  if (!api || !selection.id) return
  const src = api.getSceneElementsIncludingDeleted().find((e) => e.id === selection.id)
  if (!src || src.type !== 'text') return

  // Конвертер отличен на математике, но съедает пробелы: «Задача 5» стала бы
  // произведением курсивных переменных. Не прошло гейт — открываем пустое поле.
  const { convertAsciiMathToLatex } = await import('mathlive')
  const latex = looksLikeMath(src.text) ? convertAsciiMathToLatex(src.text) : ''

  const rect = wrapRef.current?.getBoundingClientRect()
  setMathEditor({
    elementId: null,
    latex,
    anchor: { left: (rect?.width ?? 0) / 2 - 180, top: (rect?.height ?? 0) - 180 },
    // формула встаёт ровно на место текста и остаётся в его группе и фрейме
    place: {
      x: src.x,
      y: src.y,
      angle: src.angle,
      groupIds: [...src.groupIds],
      frameId: src.frameId,
    },
  })
  // старый текст убираем тумбстоуном, иначе он вернётся к пирам через reconcile
  api.updateScene({
    elements: api
      .getSceneElementsIncludingDeleted()
      .map((e) => (e.id === src.id ? newElementWith(e, { isDeleted: true }) : e)),
    captureUpdate: CaptureUpdateAction.IMMEDIATELY,
  })
}, [selection])
```

Обратное превращение — формула становится обычным текстом с её LaTeX:

```tsx
const convertFormulaToText = useCallback(() => {
  const api = apiRef.current
  if (!api || !selection.id) return
  const elements = api.getSceneElementsIncludingDeleted()
  const src = elements.find((e) => e.id === selection.id)
  const formula = readFormula(src)
  if (!src || !formula) return

  const [text] = convertToExcalidrawElements([
    {
      type: 'text',
      x: src.x,
      y: src.y,
      text: formula.latex,
      angle: src.angle,
      groupIds: [...src.groupIds],
      frameId: src.frameId,
      strokeColor: (src.customData?.color as string) ?? api.getAppState().currentItemStrokeColor,
    },
  ])

  api.updateScene({
    elements: elements
      .map((e) => (e.id === src.id ? newElementWith(e, { isDeleted: true }) : e))
      .concat(text),
    captureUpdate: CaptureUpdateAction.IMMEDIATELY,
  })
}, [selection])
```

Рендер компонента внутри обёртки:

```tsx
<MathShapeAction
  revision={panelRevision}
  container={wrapRef.current}
  mode={selection.mode}
  onConvert={() => void convertTextToFormula()}
  onEdit={() => {
    const api = apiRef.current
    const el = api?.getSceneElements().find((x) => x.id === selection.id)
    const formula = readFormula(el)
    if (!el || !formula) return
    const rect = wrapRef.current?.getBoundingClientRect()
    setMathEditor({
      elementId: el.id,
      latex: formula.latex,
      anchor: { left: (rect?.width ?? 0) / 2 - 180, top: (rect?.height ?? 0) - 180 },
      place: { x: el.x, y: el.y, angle: el.angle, groupIds: [...el.groupIds], frameId: el.frameId },
    })
  }}
  onToText={() => void convertFormulaToText()}
/>
```

- [ ] **Step 5: Проверить сборку**

Run: `cd frontend && npx tsc --noEmit && npm run lint`
Expected: без ошибок.

- [ ] **Step 6: Ручная проверка**

- выделен текст → в панели свойств рядом с обводкой и шрифтом появилась «∑ В формулу»;
- выделена формула → «∑ Изменить» и «В текст»;
- снять выделение и выделить снова — кнопка на месте (портал пережил пересоздание панели);
- выделены два элемента — кнопки нет.

- [ ] **Step 7: Коммит**

```bash
cd frontend && git add src/components/whiteboard/MathShapeAction.tsx src/components/whiteboard/mathShapeAction.test.ts src/components/whiteboard/ExcalidrawCanvas.tsx
git commit -m "feat(board): кнопка формулы в панели свойств"
```

---

### Task 7: Приёмка двумя вкладками

**Files:** правок нет, только проверка и фиксация результата.

- [ ] **Step 1: Прогнать все тесты фронта**

```bash
cd frontend && node --test src/lib/latexToSvg.test.ts \
  src/components/whiteboard/mathFormula.test.ts \
  src/components/whiteboard/useMathFiles.test.ts \
  src/components/whiteboard/mathShapeAction.test.ts \
  src/components/whiteboard/pen.test.ts \
  src/components/whiteboard/excalidrawSync.test.ts \
  src/components/whiteboard/viewportInterp.test.ts \
  src/components/whiteboard/mediaSync.test.ts
```

Expected: все PASS. Существующие тесты доски обязаны остаться зелёными.

- [ ] **Step 2: Smoke по чеклисту скилла board-change**

Две вкладки на одной доске (препод + ученик или гость по invite-ссылке):

1. Ученик видит набор формулы по мере ввода, а не в конце.
2. Маркер рисует **поверх** формулы; формула двигается, тянется, крутится.
3. Reload у обоих — формула на месте (пришла из `customData`, S3 не участвовал).
4. Ctrl+Z откатывает формулу целиком.
5. Обычная картинка по-прежнему открывает обрезку по даблклику.
6. Формула внутри группы и внутри фрейма после конвертации из них не выпала.
7. Рисование, курсоры, follow и PDF-импорт работают как раньше.
8. В Network нет запросов к CDN (`unpkg`, `cdn.jsdelivr`) от MathLive/MathJax.

- [ ] **Step 3: Зафиксировать результат**

Если что-то из чеклиста не прошло — чинить в рамках соответствующей задачи, а не заводить новую.

```bash
cd frontend && git commit --allow-empty -m "test(board): smoke-приёмка формул пройдена"
```
