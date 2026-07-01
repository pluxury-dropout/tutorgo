import { test } from 'node:test'
import assert from 'node:assert/strict'
import { themeTokens, layoutForCount } from './callTheme.ts'

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

test('themeTokens: dark и light дают разные panel/accent', () => {
  assert.equal(themeTokens('dark').panel, '#222222')
  assert.equal(themeTokens('light').panel, '#FFFFFF')
  assert.equal(themeTokens('dark').accent, '#6CA6E0')
  assert.equal(themeTokens('light').accent, '#1D4ED8')
})
