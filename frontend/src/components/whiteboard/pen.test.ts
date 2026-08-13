// Перо собрано из трёх чужих величин: size и thinning захардкожены в бандле
// Excalidraw, шкала скорости — внутри perfect-freehand (1.2.0). Все три правит
// scripts/excalidraw-patch.mjs на postinstall, значения живут в
// scripts/pen-config.mjs. Апгрейд любого из пакетов ничего не сломает громко:
// перо просто тихо вернёт себе чужое поведение. Поэтому пиним и конфигурацию,
// и поведение.
//
// Тест читает НАСТОЯЩИЙ бандл и импортирует НАСТОЯЩУЮ perfect-freehand — то,
// чем рисует доска, а не копию опций. Отсюда же второй смысл: он падает, если
// константы покрутили, а `npm run excalidraw-patch` прогнать забыли.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readdir, readFile } from 'node:fs/promises'
import { getStrokePoints, getStrokeOutlinePoints } from 'perfect-freehand'
import { PEN_SCALE, THINNING, SPEED_SCALE } from '../../../scripts/pen-config.mjs'

const DEV = 'node_modules/@excalidraw/excalidraw/dist/dev'

// Опции getFreeDrawSvgPath: size и thinning — из бандла, остальное там жёстко.
const bundledOptions = async () => {
  for (const f of (await readdir(DEV)).filter((f) => f.endsWith('.js'))) {
    const m = (await readFile(`${DEV}/${f}`, 'utf8')).match(
      /size: \w+\.strokeWidth \* ([\d.]+),\s*thinning: ([\d.]+),/,
    )
    if (m) return { scale: Number(m[1]), thinning: Number(m[2]) }
  }
  throw new Error(`getFreeDrawSvgPath не найден в ${DEV} — сверь scripts/excalidraw-patch.mjs`)
}

const opts = (size: number, thinning: number) => ({
  size,
  thinning,
  smoothing: 0.5,
  streamline: 0.5,
  easing: (t: number) => Math.sin((t * Math.PI) / 2),
  last: true,
  simulatePressure: true,
})

// Горизонтальный штрих; шаг между точками = скорость указателя (px на событие).
// Ширину меряем в середине: концы скругляются в любом режиме (`last`).
const midWidth = (step: number, size: number, thinning: number) => {
  const o = opts(size, thinning)
  const pts: number[][] = []
  for (let x = 0; x <= 1200; x += step) pts.push([x, 100])
  const out = getStrokeOutlinePoints(getStrokePoints(pts, o), o)
  const mid = out.filter(([x]) => x > 400 && x < 800)
  return 2 * Math.max(...mid.map(([, y]) => Math.abs(y - 100)))
}

test('бандл собран с текущим pen-config', async () => {
  const { scale, thinning } = await bundledOptions()
  assert.equal(scale, PEN_SCALE, 'size в бандле разошёлся с PEN_SCALE')
  assert.equal(thinning, THINNING, 'thinning в бандле разошёлся с THINNING')
})

// Скорости по разные стороны SPEED_SCALE: обе ЗАВЕДОМО больше тонкого size —
// иначе без патча тонкое перо тоже слегка виляет, и проверка ничего не ловит.
//
// Абсолютной нижней границы ширины тут нет: она целиком следует из THINNING
// (быстрый штрих = size × easing(0.5 − THINNING/2)) и потому пинила бы вкусовую
// калибровку, а не поведение. Тонко/толсто решается глазом на доске.
const SLOW = SPEED_SCALE / 5
const FAST = SPEED_SCALE * 1.2

// strokeWidth из тулбара — три градации.
const SIZES = [1, 2, 4].map((w) => w * PEN_SCALE)

test('толщина следует за скоростью так, как задано в pen-config', () => {
  if (THINNING === 0) {
    // Ровное перо: pressure-путь обойдён, радиус берётся как size/2.
    for (const size of SIZES) {
      for (const [name, step] of [['медленно', SLOW], ['быстро', FAST]] as const) {
        const w = midWidth(step, size, THINNING)
        assert.ok(Math.abs(w - size) < 0.01, `size ${size}, ${name}: ширина ${w}`)
      }
    }
    return
  }

  const ratios = SIZES.map((size) => {
    const slow = midWidth(SLOW, size, THINNING)
    const fast = midWidth(FAST, size, THINNING)
    assert.ok(slow > fast, `size ${size}: медленный штрих не толще быстрого (${slow} vs ${fast})`)
    return slow / fast
  })

  // Суть патча SPEED_SCALE: профиль «скорость → толщина» ОДИН для всех градаций
  // тулбара. В оригинале шкалой служит сам size, и тогда тонкое перо насыщается
  // в «всегда быстро» (ratio ≈ 1.0), пока толстое ещё виляет (ratio ≈ 2.0) —
  // расхождение и ловим. Проверка не зависит от калибровки: она сравнивает
  // градации между собой, а не с эталонными числами.
  const spread = Math.max(...ratios) - Math.min(...ratios)
  assert.ok(
    spread < 0.01,
    `профиль скорости зависит от толщины (${ratios.map((r) => r.toFixed(3)).join(', ')}) — ` +
      'не применён патч SPEED_SCALE в perfect-freehand?',
  )
})
