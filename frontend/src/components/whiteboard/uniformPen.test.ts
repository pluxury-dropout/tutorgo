// «Ровное перо» держится на одном свойстве perfect-freehand: при
// simulatePressure:false толщина считается из pressures[], а мышь всегда шлёт
// 0.5 → линия ровная. Само свойство — чужое (perfect-freehand 1.2.0, вложен в
// @excalidraw/excalidraw), и его смена при апгрейде ничего не сломает громко:
// перо просто тихо станет обычным. Поэтому пиним поведение тестом.
//
// Опции повторяют getFreeDrawSvgPath из dist/*/chunk-*.js — если разойдутся,
// тест начнёт мерить не то, что рисует доска.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { getStrokePoints, getStrokeOutlinePoints } from 'perfect-freehand'

const opts = (size: number, simulatePressure: boolean) => ({
  size,
  thinning: 0.6,
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

// Полуширина в середине штриха: концы сужаются в обоих режимах (`last`).
const midWidth = (step: number, size: number, simulatePressure: boolean) => {
  const o = opts(size, simulatePressure)
  const pts = stroke(step)
  const input = simulatePressure ? pts : pts.map(([x, y]) => [x, y, 0.5])
  const out = getStrokeOutlinePoints(getStrokePoints(input, o), o)
  const mid = out.filter(([x]) => x > 200 && x < 400)
  return Math.max(...mid.map(([, y]) => Math.abs(y - 100)))
}

const SIZES = [1, 2, 4].map((strokeWidth) => strokeWidth * 4.25)

test('ровное перо: толщина не зависит от скорости', () => {
  for (const size of SIZES) {
    const slow = midWidth(2, size, false)
    const fast = midWidth(40, size, false)
    assert.ok(
      Math.abs(slow / fast - 1) < 0.02,
      `size ${size}: медленно ${slow} ≠ быстро ${fast}`,
    )
  }
})

test('обычное перо: толщина зависит от скорости', () => {
  // Контроль: без него первый тест прошёл бы и на пере, которое ровное просто
  // потому, что эффекта скорости нет вообще (так и было при size ≈ 2px —
  // min(1, distance/size) насыщается, и толщина залипает).
  for (const size of SIZES) {
    const slow = midWidth(2, size, true)
    const fast = midWidth(40, size, true)
    assert.ok(slow / fast > 1.2, `size ${size}: скорость не влияет (${slow} vs ${fast})`)
  }
})
