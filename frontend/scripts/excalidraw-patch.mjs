// Правит бандлы в node_modules: перо (Excalidraw + perfect-freehand) и предел
// отдаления.
//
// Зачем патч, а не пропсы: все эти величины захардкожены внутри пакетов и наружу
// не отдаются.
//
//   1. Перо — три ручки в scripts/pen-config.mjs, физика описана там же. Две
//      живут в getFreeDrawSvgPath у Excalidraw (size, thinning), третья —
//      в самой perfect-freehand (шкала скорости, в оригинале это size).
//      size тоньшим здесь, а не через appState.currentItemStrokeWidth, потому
//      что strokeWidth общий для всех инструментов: там 0.25 дало бы волосяные
//      прямоугольники и стрелки. Тулбар продолжает работать, три его градации
//      просто становятся тоньше.
//
//   2. MIN_ZOOM объявлен один раз и импортируется во все места клампа (колесо,
//      пинч, кнопка «−», zoom-to-fit), поэтому хватает одной строки.
//
//   3. Зум у Excalidraw аддитивный: кнопки дают zoom ± 0.1, колесо —
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
//
//   4. Новый текст (даблклик по пустому месту, инструмент «Текст») Excalidraw
//      кладёт верхним левым углом в курсор, поэтому поле «вываливается» вниз
//      из-под указателя. Сдвигаем на полстроки вверх — на курсоре оказывается
//      середина первой строки. Ветка снапа к центру контейнера не трогается:
//      там позицию даёт parentCenterPosition.
//
//   5. Рамка выделения у Excalidraw ловит только те элементы, чей bbox лежит
//      в ней ЦЕЛИКОМ (containment). В Miro/Figma достаточно задеть элемент
//      краем (intersection) — иначе, чтобы забрать длинную линию или крупную
//      картинку, приходится отдалять доску и обводить её целиком. Замена —
//      те же четыре сравнения, только углы перекрёстные: два AABB
//      пересекаются, когда sel.x1 ≤ el.x2 ∧ sel.x2 ≥ el.x1 и то же по Y.
//      Тест по bbox, а не по геометрии штриха: у Excalidraw и хит-тест
//      выделенной группы работает по общему bbox, так что рамка ведёт себя
//      с ним согласованно.
import { readdir, readFile, writeFile } from 'node:fs/promises'
import { PEN_SCALE, THINNING, SPEED_SCALE } from './pen-config.mjs'

// ponytail: замена по якорям в dist, а не patch-package и не форк.
// patch-package хранит текстовый diff, а правим мы МИНИФИЦИРОВАННЫЙ бандл с
// хешем в имени чанка — такой diff разваливается от любой пересборки апстрима,
// тогда как якорь-регулярка переживает и переминификацию, и повторный прогон.
// Число патчей само по себе не повод форкать: пока каждый — это замена
// «выражение → выражение», форк дороже (монорепо Excalidraw, свой билд в CI,
// ребейз на каждый релиз) и вдобавок теряет проверку hits, которая роняет
// postinstall при съехавшем якоре.
// Форкать, когда: (1) нужен НОВЫЙ код внутри редактора — свой инструмент, поле
// элемента, попап: такое бандлом не патчится вообще; (2) один апгрейд ломает
// 3+ якоря разом; (3) понадобился доступ к @excalidraw/element и соседям как
// к API.
const DIST = 'node_modules/@excalidraw/excalidraw/dist'

// 1% — примерно как отдаление в Miro (штатный предел Excalidraw — 10%).
const MIN_ZOOM = 0.01

// Bundler'ы минифицируют по-разному, поэтому цели описаны дважды: dev-сборку
// берёт `next dev` (exports condition "development"), prod — сборка.
//
// Правки пера заданы регулярками, а не строками: значения крутят вручную, и
// патч обязан переписывать уже пропатченный бандл — иначе каждая итерация
// калибровки требовала бы переустановки пакета. `[\d.]+` на месте числа как раз
// это и даёт. Строковые правки зума не итерируют, там хватает точных якорей.
const excalidrawFiles = (
  await Promise.all(
    ['dev', 'prod'].map(async (dir) =>
      (await readdir(`${DIST}/${dir}`))
        .filter((f) => f.endsWith('.js'))
        .map((f) => `${DIST}/${dir}/${f}`)
    )
  )
).flat()

// Единственная копия в дереве (1.2.0), её импортируют оба чанка Excalidraw.
const freehandFiles = ['esm', 'cjs'].map(
  (d) => `node_modules/perfect-freehand/dist/${d}/index.js`
)

// hits — сколько попаданий ждём суммарно по files. Меньше или больше — падаем:
// значит апгрейд пакета уехал по якорю, и молчаливый no-op тихо вернул бы перу
// чужое поведение.
const EDITS = [
  { from: /size: ?(\w+)\.strokeWidth ?\* ?[\d.]+/g, to: `size: $1.strokeWidth * ${PEN_SCALE}`, hits: 2 },
  { from: /thinning: ?[\d.]+,/g, to: `thinning: ${THINNING},`, hits: 2 },

  // Обе точки, где симулированный нажим считается из скорости: инициализация по
  // первым 10 точкам и основной цикл. Имена переменных в esm и cjs разные,
  // поэтому якорь — форма выражения `let X=C(1,ЧТО-ТО/size),Y=C(1,1-X)`, а не
  // конкретный идентификатор. В знаменателе `\w+` — оно же матчит и уже
  // подставленное число, отсюда идемпотентность.
  {
    from: /let (\w+)=C\(1,([\w.]+)\/\w+\),(\w+)=C\(1,1-\1\)/g,
    to: `let $1=C(1,$2/${SPEED_SCALE}),$3=C(1,1-$1)`,
    files: freehandFiles,
    hits: 4,
  },

  { from: 'var MIN_ZOOM = 0.1;', to: `var MIN_ZOOM = ${MIN_ZOOM};` },
  // Соседи по var-блоку (ZOOM_STEP, MAX_ZOOM) держат якорь уникальным: сам по
  // себе `=.1` в бандле встречается и в другом смысле.
  { from: ',Js=.1,Ys=30', to: `,Js=${MIN_ZOOM},Ys=30` },

  // Кнопки «+»/«−» и Ctrl+±: ZOOM_STEP (0.1) из слагаемого — в множитель.
  { from: 'appState.zoom.value + ZOOM_STEP', to: 'appState.zoom.value * (1 + ZOOM_STEP)' },
  { from: 'appState.zoom.value - ZOOM_STEP', to: 'appState.zoom.value / (1 + ZOOM_STEP)' },
  { from: 'o.zoom.value+Fn', to: 'o.zoom.value*(1+Fn)' },
  { from: 'o.zoom.value-Fn', to: 'o.zoom.value/(1+Fn)' },

  // Колесо с Ctrl.
  { from: 'this.state.zoom.value - delta / 100', to: 'this.state.zoom.value * Math.exp(-delta / 100)' },
  { from: 'this.state.zoom.value-s/100', to: 'this.state.zoom.value*Math.exp(-s/100)' },
  // Обнуление амплификации: множитель остаётся мёртвым слагаемым `+= 0 * …`,
  // зато якорь короткий и переживёт переминификацию соседей. Комментарий в
  // замене нужен скрипту: голый `0` встречается везде, и проверка «уже
  // пропатчен» перестала бы отличать патч от съехавшего после апгрейда якоря.
  { from: 'Math.log10(Math.max(1, this.state.zoom.value))', to: '0 /* ponytail: zoom-амплификация */' },
  { from: 'Math.log10(Math.max(1,this.state.zoom.value))', to: '0/*ponytail:zoom*/' },

  // Высота строки в Excalidraw = fontSize × lineHeight (getLineHeightInPx),
  // половина её и есть сдвиг. В prod-бандле те же величины зовутся u и p,
  // sceneY — r: имена короткие, поэтому якорем служит всё выражение целиком.
  {
    from: 'y: parentCenterPosition ? parentCenterPosition.elementCenterY : sceneY,',
    to: 'y: parentCenterPosition ? parentCenterPosition.elementCenterY : sceneY - fontSize * lineHeight / 2,',
  },
  { from: 'y:s?s.elementCenterY:r,', to: 'y:s?s.elementCenterY:r-u*p/2,' },

  // getElementsWithinSelection: containment → intersection (см. п. 5).
  // В prod-бандле рамка — [o,i,a,s], элемент — [l,U,p,m]; соседние `&&`
  // держат якорь уникальным, голая цепочка сравнений слишком общая.
  {
    from: 'selectionX1 <= elementX1 && selectionY1 <= elementY1 && selectionX2 >= elementX2 && selectionY2 >= elementY2',
    to: 'selectionX1 <= elementX2 && selectionX2 >= elementX1 && selectionY1 <= elementY2 && selectionY2 >= elementY1',
  },
  { from: '&&o<=l&&i<=U&&a>=p&&s>=m', to: '&&o<=p&&a>=l&&i<=m&&s>=U' },
]

const count = (s, from) =>
  typeof from === 'string' ? s.split(from).length - 1 : (s.match(from) || []).length

// Только .js: рядом лежат .map с тем же текстом, их правка ничего не даёт.
// Имена чанков содержат хеш и при апгрейде пакета меняются, поэтому ищем по
// всей папке, а не по фиксированному пути.
const src = new Map()
for (const f of [...excalidrawFiles, ...freehandFiles]) src.set(f, await readFile(f, 'utf8'))

const dirty = new Set()

for (const { from, to, files = excalidrawFiles, hits = 1 } of EDITS) {
  const total = files.reduce((n, f) => n + count(src.get(f), from), 0)

  if (total === 0) {
    // Идемпотентны только регулярки; строковую правку зума узнаём по результату.
    if (typeof from === 'string' && files.some((f) => src.get(f).includes(to))) {
      console.log(`[excalidraw] уже пропатчен: ${to}`)
      continue
    }
    console.error(`[excalidraw] не найдено: ${from}`)
    console.error('[excalidraw] похоже на апгрейд пакета — сверь якорь с dist/')
    process.exit(1)
  }
  if (total !== hits) {
    console.error(`[excalidraw] ${from} — попаданий ${total}, ожидалось ${hits}; сверь якорь с dist/`)
    process.exit(1)
  }

  for (const f of files.filter((f) => count(src.get(f), from) > 0)) {
    src.set(f, src.get(f).replace(from, to))
    dirty.add(f)
    console.log(`[excalidraw] ${f} ← ${to}`)
  }
}

for (const f of dirty) await writeFile(f, src.get(f))
