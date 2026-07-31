import { test } from 'node:test'
import assert from 'node:assert/strict'
import { CALL_THEME, layoutForCount } from './callTheme.ts'

test('layoutForCount: 0 и 1 участник → single', () => {
  assert.deepEqual(layoutForCount(0), { mode: 'single', columns: 1 })
  assert.deepEqual(layoutForCount(1), { mode: 'single', columns: 1 })
})

test('layoutForCount: 2 участника → сетка 2 колонки (кейс 1:1 урока)', () => {
  assert.deepEqual(layoutForCount(2), { mode: 'grid', columns: 2 })
})

test('layoutForCount: 3+ участников → сетка 3 колонки', () => {
  assert.deepEqual(layoutForCount(3), { mode: 'grid', columns: 3 })
  assert.deepEqual(layoutForCount(5), { mode: 'grid', columns: 3 })
})

// Тема звонка обязана ссылаться на общие токены: хардкод цвета здесь означает
// вторую копию палитры, которая рано или поздно разъедется с globals.css.
test('CALL_THEME: все значения — ссылки на CSS-переменные, без хардкода цветов', () => {
  for (const [key, value] of Object.entries(CALL_THEME)) {
    assert.ok(
      value.includes('var(--'),
      `${key} = ${value} — ожидалась ссылка на токен globals.css`
    )
  }
})
