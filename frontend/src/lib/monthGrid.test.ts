import { test } from 'node:test'
import assert from 'node:assert/strict'
import { buildGrid, isSameLocalDay, shiftMonth } from './monthGrid.ts'

test('сетка начинается с понедельника', () => {
  // 1 августа 2026 — суббота: пять пустых клеток перед ней.
  const grid = buildGrid(2026, 7)
  assert.equal(grid.slice(0, 5).filter((c) => c === null).length, 5)
  assert.equal(grid[5]?.getDate(), 1)
  assert.equal(grid.length, 5 + 31)
})

test('месяц, начинающийся с понедельника, идёт без заглушек', () => {
  // 1 июня 2026 — понедельник.
  const grid = buildGrid(2026, 5)
  assert.equal(grid[0]?.getDate(), 1)
  assert.equal(grid.length, 30)
})

test('воскресенье первого числа даёт шесть заглушек, а не ноль', () => {
  // 1 февраля 2026 — воскресенье, последний день ISO-недели.
  const grid = buildGrid(2026, 1)
  assert.equal(grid.slice(0, 6).filter((c) => c === null).length, 6)
  assert.equal(grid[6]?.getDate(), 1)
})

test('високосный февраль вмещает 29 дней', () => {
  assert.equal(buildGrid(2028, 1).filter(Boolean).length, 29)
})

test('shiftMonth обрезает день по длине месяца', () => {
  const march31 = new Date(2026, 2, 31)
  const feb = shiftMonth(march31, -1)
  assert.equal(feb.getMonth(), 1)
  assert.equal(feb.getDate(), 28) // не 3 марта из-за переполнения
})

test('shiftMonth переходит через границу года', () => {
  const jan = shiftMonth(new Date(2026, 0, 15), -1)
  assert.equal(jan.getFullYear(), 2025)
  assert.equal(jan.getMonth(), 11)
  assert.equal(jan.getDate(), 15)
})

test('shiftMonth отдаёт полночь, а не время исходной даты', () => {
  const d = shiftMonth(new Date(2026, 7, 12, 16, 30), 1)
  assert.equal(d.getHours(), 0)
  assert.equal(d.getMinutes(), 0)
})

test('isSameLocalDay сравнивает день, а не момент', () => {
  assert.ok(isSameLocalDay(new Date(2026, 7, 12, 0, 1), new Date(2026, 7, 12, 23, 59)))
  assert.ok(!isSameLocalDay(new Date(2026, 7, 12), new Date(2026, 8, 12)))
  assert.ok(!isSameLocalDay(new Date(2025, 7, 12), new Date(2026, 7, 12)))
})
