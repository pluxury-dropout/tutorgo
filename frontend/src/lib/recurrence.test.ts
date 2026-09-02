process.env.TZ = 'Asia/Almaty'   // isoWeekday читает локальную зону — фиксируем её

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { toRecurrenceInput, isoWeekday } from './recurrence.ts'

test('воскресенье — седьмой день, а не нулевой', () => {
  assert.equal(isoWeekday(new Date('2026-09-01T17:00:00+05:00')), 2)  // вторник
  assert.equal(isoWeekday(new Date('2026-09-06T17:00:00+05:00')), 7)  // воскресенье
  assert.equal(isoWeekday(new Date('2026-09-07T17:00:00+05:00')), 1)  // понедельник
})

test('weekly_same превращается в правило без дней и без интервала', () => {
  const rule = toRecurrenceInput({ type: 'weekly_same' })

  assert.equal(rule.freq, 'weekly')
  assert.equal(rule.byweekday, undefined)   // день берётся из первого вхождения
  assert.equal(rule.interval_n, undefined)
  assert.equal(rule.max_count, undefined)   // бессрочно, горизонт тянет джоба
})

test('weekly_custom передаёт выбранные дни недели', () => {
  const rule = toRecurrenceInput({ type: 'weekly_custom', days: [1, 3] })

  assert.deepEqual(rule.byweekday, [1, 3])
})

test('every_n_weeks передаёт интервал, по умолчанию через неделю', () => {
  assert.equal(toRecurrenceInput({ type: 'every_n_weeks', n: 3 }).interval_n, 3)
  assert.equal(toRecurrenceInput({ type: 'every_n_weeks' }).interval_n, 2)
})

test('количество уроков едет в max_count', () => {
  assert.equal(toRecurrenceInput({ type: 'weekly_same', count: 20 }).max_count, 20)
})
