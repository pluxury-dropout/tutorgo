// Раскладывает шрифты Excalidraw в public/fonts и подменяет их своими.
//
// Зачем: без EXCALIDRAW_ASSET_PATH Excalidraw тянет woff2 с unpkg.com при
// каждом открытии доски. Раздаём их сами (см. ExcalidrawCanvas.tsx).
//
// Как добавить свой шрифт: положить fonts/<Слот>.woff2 и запустить `npm run fonts`.
// Гарнитура ЗАМЕНЯЕТ содержимое слота, а не добавляет новый пункт в пикер:
// Excalidraw хранит fontFamily в элементе числом (ссылка на слот), поэтому
// подмена байтов автоматически даёт один и тот же шрифт у препода, ученика и
// гостя — без реестра в БД и без миграции старых досок.
import { cp, readdir, copyFile, rm } from 'node:fs/promises'
import { existsSync, readFileSync } from 'node:fs'
import path from 'node:path'

const DIST = 'node_modules/@excalidraw/excalidraw/dist/prod/fonts'
const OUT = 'public/fonts'
const CUSTOM = 'fonts'

// ponytail: Xiaolai (13 МБ, CJK) пропущен — уроки на русском, а он один весит
// больше всех остальных вместе. Понадобится китайский — дописать сюда; без него
// иероглифы просто упадут на системный шрифт (404 на woff2 не ломает доску).
const SLOTS = [
  'Excalifont', // «От руки»
  'Nunito', // «Обычный» — дефолт
  'ComicShanns', // «Код»
  'Lilita',
  'Virgil',
  'Assistant',
  'Cascadia',
  'Liberation',
]

await rm(OUT, { recursive: true, force: true })

for (const slot of SLOTS) {
  const dest = path.join(OUT, slot)
  await cp(path.join(DIST, slot), dest, { recursive: true })

  const custom = path.join(CUSTOM, `${slot}.woff2`)
  if (!existsSync(custom)) continue

  // Слот нарезан на сабсеты (latin, cyrillic, greek…), у каждого свой файл с
  // хешем в имени, зашитый в бандл Excalidraw. Кладём свою гарнитуру целиком
  // под каждым именем: unicodeRange в @font-face сам решит, какие глифы брать
  // из какого файла, так что лишние копии безвредны.
  for (const subset of await readdir(dest)) {
    await copyFile(custom, path.join(dest, subset))
  }
  console.log(`[fonts] ${slot} ← ${custom}`)
}

// Имена woff2 содержат хеш и зашиты в бандл Excalidraw — при апгрейде пакета
// они сменятся, и доска молча уедет на системный шрифт (404 не падает громко).
// Сверяем то, на что ссылается бандл, с тем, что реально лежит в public.
const bundle = (await readdir(path.dirname(DIST)))
  .filter((f) => f.endsWith('.js'))
  .map((f) => readFileSync(path.join(path.dirname(DIST), f), 'utf8'))
  .join('')

const missing = [...new Set([...bundle.matchAll(/\.\/fonts\/([^"]+\.woff2)/g)].map((m) => m[1]))]
  .filter((ref) => SLOTS.some((s) => ref.startsWith(`${s}/`)))
  .filter((ref) => !existsSync(path.join(OUT, ref)))

if (missing.length) {
  console.error(`[fonts] бандл ссылается на ${missing.length} отсутствующих woff2, напр.:`)
  console.error(`  ${missing[0]}`)
  console.error('[fonts] похоже на апгрейд @excalidraw/excalidraw — сверь SLOTS с dist/prod/fonts')
  process.exit(1)
}

// ponytail: метрики (unitsPerEm/ascender/descender/lineHeight) захардкожены в
// бандле Excalidraw по слотам, из подменённого woff2 они не читаются. Своя
// гарнитура с сильно другими метриками поедет по высоте строки — если станет
// заметно, переписывать метрики в woff2 здесь же через fontTools.
console.log(`[fonts] ${SLOTS.length} слотов → ${OUT}`)
