import { test } from 'node:test'
import assert from 'node:assert/strict'
import { dropSlot, HOUR_PX, GUTTER_PX, HOURS } from './weekGridDrop.ts'

// Сетка как на телефоне: 390px ширины, верх сетки на 100px от верха экрана.
const grid = { left: 0, top: 100, width: 390 }
const colW = (grid.width - GUTTER_PX) / 7

/** Блок шириной в колонку, левый край — в колонке day, верх — в часе hour. */
const ghostAt = (day: number, hour: number, minute = 0) => ({
  left:  GUTTER_PX + day * colW,
  top:   grid.top + ((hour - HOURS[0]) * 60 + minute) / 60 * HOUR_PX,
  width: colW,
})

test('дроп по центру блока попадает в свою колонку', () => {
  for (let d = 0; d < 7; d++) {
    assert.equal(dropSlot(ghostAt(d, 10), grid, 60).day, d, `колонка ${d}`)
  }
})

test('время снапится к получасу', () => {
  assert.equal(dropSlot(ghostAt(0, 10, 8),  grid, 60).minutes, 10 * 60)      // 10:08 → 10:00
  assert.equal(dropSlot(ghostAt(0, 10, 22), grid, 60).minutes, 10 * 60 + 30) // 10:22 → 10:30
  assert.equal(dropSlot(ghostAt(0, 10, 50), grid, 60).minutes, 11 * 60)      // 10:50 → 11:00
})

test('бросок выше сетки прижимается к первому часу, ниже — к последнему слоту', () => {
  assert.equal(dropSlot({ ...ghostAt(0, 10), top: -500 }, grid, 60).minutes, HOURS[0] * 60)
  // сетка кончается в 22:00, часовой урок дальше 21:00 не встанет
  assert.equal(dropSlot({ ...ghostAt(0, 10), top: 5000 }, grid, 60).minutes, 21 * 60)
  assert.equal(dropSlot({ ...ghostAt(0, 10), top: 5000 }, grid, 30).minutes, 21 * 60 + 30)
})

test('бросок за края сетки остаётся в неделе', () => {
  assert.equal(dropSlot({ ...ghostAt(0, 10), left: -300 }, grid, 60).day, 0)
  assert.equal(dropSlot({ ...ghostAt(6, 10), left: 900 },  grid, 60).day, 6)
})

test('гуттер часов не считается колонкой: блок у самой левой границы — понедельник', () => {
  assert.equal(dropSlot({ left: 0, top: grid.top, width: colW }, grid, 60).day, 0)
})
