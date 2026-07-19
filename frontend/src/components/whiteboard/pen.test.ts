// Перо держится на одном свойстве perfect-freehand: при thinning:0 радиус
// штриха берётся как size/2 и весь pressure-путь (а для мыши «нажим» —
// это скорость) обходится стороной. Свойство чужое (perfect-freehand 1.2.0,
// вложен в @excalidraw/excalidraw), и его смена при апгрейде ничего не сломает
// громко: перо просто тихо начнёт вилять толщиной. Поэтому пиним поведение.
//
// Сам факт, что бандл пропатчен, проверяет scripts/excalidraw-patch.mjs — он
// падает на postinstall, если строка в dist уехала.
//
// Опции повторяют getFreeDrawSvgPath из dist/*/chunk-*.js — если разойдутся,
// тест начнёт мерить не то, что рисует доска.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { getStrokePoints, getStrokeOutlinePoints } from 'perfect-freehand'

const opts = (size: number, thinning: number, simulatePressure: boolean) => ({
  size,
  thinning,
  smoothing: 0.5,
  streamline: 0.5,
  easing: (t: number) => Math.sin((t * Math.PI) / 2),
  last: true,
  simulatePressure,
})

// Горизонтальный штрих; шаг между точками = скорость мыши.
const stroke = (step: number) => {
  const pts: number[][] = []
  for (let x = 0; x <= 600; x += step) pts.push([x, 100])
  return pts
}

// Ширина в середине штриха: концы скругляются в любом режиме (`last`).
const midWidth = (step: number, size: number, thinning: number, simulatePressure: boolean) => {
  const o = opts(size, thinning, simulatePressure)
  const pts = stroke(step)
  const input = simulatePressure ? pts : pts.map(([x, y]) => [x, y, 0.5])
  const out = getStrokeOutlinePoints(getStrokePoints(input, o), o)
  const mid = out.filter(([x]) => x > 200 && x < 400)
  return 2 * Math.max(...mid.map(([, y]) => Math.abs(y - 100)))
}

// strokeWidth из тулбара (1/2/4) × PEN_SCALE из scripts/excalidraw-patch.mjs.
const SIZES = [1, 2, 4].map((strokeWidth) => strokeWidth * 1.5)

test('thinning:0 — ширина ровно size, что бы ни делали мышь и стилус', () => {
  for (const size of SIZES) {
    for (const simulatePressure of [true, false]) {
      for (const step of [2, 40]) {
        const w = midWidth(step, size, 0, simulatePressure)
        assert.ok(
          Math.abs(w - size) < 0.01,
          `size ${size}, шаг ${step}, simulatePressure ${simulatePressure}: ширина ${w}`,
        )
      }
    }
  }
})

test('контроль: при thinning>0 скорость влияет на толщину', () => {
  // Без этого теста первый прошёл бы и на пере, у которого эффекта скорости
  // нет вообще — например, если бы getStrokeOutlinePoints перестал читать
  // pressure и «ровно» стало бы ровным по случайности, а не по thinning:0.
  //
  // Меряем на штатных 4.25: имитация нажима считается как min(1, distance/size)
  // и на наших тонких размерах насыщается — виляние там и без патча почти не
  // видно. Контролю нужен размер, на котором эффект заведомо есть.
  const slow = midWidth(2, 4.25, 0.6, true)
  const fast = midWidth(40, 4.25, 0.6, true)
  assert.ok(slow / fast > 1.2, `скорость не влияет (${slow} vs ${fast})`)
})
