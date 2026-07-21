// Правит бандл Excalidraw в node_modules: перо и предел отдаления.
//
// Зачем патч, а не пропсы: обе величины захардкожены внутри пакета и наружу не
// отдаются.
//
//   1. thinning — perfect-freehand сужает штрих по «нажиму», а для мыши нажим
//      считается из скорости, отсюда виляющая толщина. thinning:0 выключает
//      это целиком: радиус берётся как size/2, минуя весь pressure-путь.
//      Поэтому же ушёл прежний патч simulatePressure — при нулевом thinning
//      давление на толщину не влияет вообще, и «ровное перо» стало не режимом,
//      а единственным поведением.
//
//   2. size = strokeWidth * 4.25 — множитель, живущий только в
//      getFreeDrawSvgPath. Тоньшим его, а не appState.currentItemStrokeWidth,
//      потому что strokeWidth общий для всех инструментов: там 0.25 дало бы
//      волосяные прямоугольники и стрелки. Тулбар продолжает работать, три
//      его градации просто становятся тоньше.
//
//   3. MIN_ZOOM объявлен один раз и импортируется во все места клампа (колесо,
//      пинч, кнопка «−», zoom-to-fit), поэтому хватает одной строки.
//
//   4. Зум у Excalidraw аддитивный: кнопки дают zoom ± 0.1, колесо —
//      zoom − delta/100. Шаг в абсолютных единицах, поэтому на 100% он
//      ощущается как 10%, а на 10% — как 100%, и последний шаг вниз падает
//      прямо в MIN_ZOOM: рывок 10% → 1%. Делаем шаг мультипликативным
//      (zoom × k), тогда он постоянен в логарифмическом масштабе — «скорость»
//      одинакова на любом уровне. exp(−delta/100) при zoom ≈ 1 совпадает с
//      прежним zoom − delta/100 с точностью до второго порядка, так что
//      привычное поведение около 100% сохраняется. Заодно обнуляем
//      амплификацию log10(max(1, zoom)): она существовала ровно чтобы
//      компенсировать аддитивность при сильном приближении, а поверх
//      умножения снова делает скорость неравномерной. Пинч не трогаем — он и
//      так мультипликативный (initialScale × event.scale).
import { readdir, readFile, writeFile } from 'node:fs/promises'

// ponytail: sed по dist вместо patch-package — в проекте уже есть postinstall,
// правящий node_modules (см. excalidraw-fonts.mjs). Станет патчей больше двух —
// заводить patch-package, он хранит diff в git и виден на ревью.
const DIST = 'node_modules/@excalidraw/excalidraw/dist'

// Ширина штриха = strokeWidth * PEN_SCALE. Штатное «тонкое» (strokeWidth 1) —
// это 1.5px, «жирное» (2) — 3px. Меньше 1.5 линия начинает бледнеть: freedraw
// заливается как фигура, и субпиксельная ширина уходит в антиалиасинг.
const PEN_SCALE = 1.5
// 1% — примерно как отдаление в Miro (штатный предел Excalidraw — 10%).
const MIN_ZOOM = 0.01

// Bundler'ы минифицируют по-разному, поэтому каждая цель описана дважды:
// dev-сборку берёт `next dev` (exports condition "development"), prod — сборка.
const EDITS = [
  ['size: element.strokeWidth * 4.25', `size: element.strokeWidth * ${PEN_SCALE}`],
  ['size:e.strokeWidth*4.25', `size:e.strokeWidth*${PEN_SCALE}`],
  ['thinning: 0.6,', 'thinning: 0,'],
  ['thinning:.6,', 'thinning:0,'],
  ['var MIN_ZOOM = 0.1;', `var MIN_ZOOM = ${MIN_ZOOM};`],
  // Соседи по var-блоку (ZOOM_STEP, MAX_ZOOM) держат якорь уникальным: сам по
  // себе `=.1` в бандле встречается и в другом смысле.
  [',Js=.1,Ys=30', `,Js=${MIN_ZOOM},Ys=30`],

  // Кнопки «+»/«−» и Ctrl+±: ZOOM_STEP (0.1) из слагаемого — в множитель.
  ['appState.zoom.value + ZOOM_STEP', 'appState.zoom.value * (1 + ZOOM_STEP)'],
  ['appState.zoom.value - ZOOM_STEP', 'appState.zoom.value / (1 + ZOOM_STEP)'],
  ['o.zoom.value+Fn', 'o.zoom.value*(1+Fn)'],
  ['o.zoom.value-Fn', 'o.zoom.value/(1+Fn)'],

  // Колесо с Ctrl.
  ['this.state.zoom.value - delta / 100', 'this.state.zoom.value * Math.exp(-delta / 100)'],
  ['this.state.zoom.value-s/100', 'this.state.zoom.value*Math.exp(-s/100)'],
  // Обнуление амплификации: множитель остаётся мёртвым слагаемым `+= 0 * …`,
  // зато якорь короткий и переживёт переминификацию соседей. Комментарий в
  // замене нужен скрипту: голый `0` встречается везде, и проверка «уже
  // пропатчен» перестала бы отличать патч от съехавшего после апгрейда якоря.
  ['Math.log10(Math.max(1, this.state.zoom.value))', '0 /* ponytail: zoom-амплификация */'],
  ['Math.log10(Math.max(1,this.state.zoom.value))', '0/*ponytail:zoom*/'],
]

// Только .js: рядом лежат .map с тем же текстом, их правка ничего не даёт.
// Имена чанков содержат хеш и при апгрейде пакета меняются, поэтому ищем по
// всей папке, а не по фиксированному пути.
const files = (
  await Promise.all(
    ['dev', 'prod'].map(async (dir) =>
      (await readdir(`${DIST}/${dir}`))
        .filter((f) => f.endsWith('.js'))
        .map((f) => `${DIST}/${dir}/${f}`)
    )
  )
).flat()

const src = new Map()
for (const f of files) src.set(f, await readFile(f, 'utf8'))

const dirty = new Set()

for (const [from, to] of EDITS) {
  const hits = files.filter((f) => src.get(f).includes(from))

  // Строки захардкожены в бандле и при апгрейде пакета могут уехать или
  // переминифицироваться. Без этой проверки sed молча стал бы no-op, а перо
  // тихо вернуло бы себе «нажим». Падаем громко.
  if (hits.length === 0) {
    if (files.some((f) => src.get(f).includes(to))) {
      console.log(`[excalidraw] уже пропатчен: ${to}`)
      continue
    }
    console.error(`[excalidraw] не найдено: "${from}"`)
    console.error('[excalidraw] похоже на апгрейд @excalidraw/excalidraw — сверь строку с dist/')
    process.exit(1)
  }
  if (hits.length > 1 || src.get(hits[0]).split(from).length - 1 !== 1) {
    console.error(`[excalidraw] "${from}" встречается не один раз — сверь строку с dist/`)
    process.exit(1)
  }

  src.set(hits[0], src.get(hits[0]).replace(from, to))
  dirty.add(hits[0])
  console.log(`[excalidraw] ${hits[0]} ← ${to}`)
}

for (const f of dirty) await writeFile(f, src.get(f))
