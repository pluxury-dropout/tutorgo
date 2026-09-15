import { test } from 'node:test'
import assert from 'node:assert/strict'

import { amountFor } from './money.ts'

test('кратное пакету — ровно пакеты, даже при дробной цене урока', () => {
  assert.equal(amountFor(12, 85000, 12), 85000)
  assert.equal(amountFor(24, 85000, 12), 170000)
})

test('неполный пакет — уроки × цена урока в целых тенге', () => {
  assert.equal(amountFor(5, 85000, 12), 35417)
  assert.equal(amountFor(4, 6000, 1), 24000)
})

test('ноль уроков — ноль', () => {
  assert.equal(amountFor(0, 40000, 8), 0)
})
