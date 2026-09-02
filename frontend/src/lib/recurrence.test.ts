process.env.TZ = 'Asia/Almaty'   // генерация зависит от локальной зоны — фиксируем её

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { generateDates, lessonsPlural, isoWeekday } from './recurrence.ts'

const TUE_1_SEP = new Date('2026-09-01T17:00:00+05:00').toISOString()

test('бессрочный еженедельный повтор упирается в горизонт 12 месяцев', () => {
  const dates = generateDates(TUE_1_SEP, { type: 'weekly_same' })

  assert.equal(dates.length, 53)
  const horizon = new Date('2027-09-01T17:00:00+05:00')
  assert.ok(new Date(dates[dates.length - 1]) <= horizon)
})

test('явное количество уроков горизонтом не режется', () => {
  const dates = generateDates(TUE_1_SEP, { type: 'weekly_same', count: 200 })

  assert.equal(dates.length, 200)
})

test('weekly_custom без выбранных дней даёт пусто, а не один урок', () => {
  assert.deepEqual(generateDates(TUE_1_SEP, { type: 'weekly_custom', days: [] }), [])
})

test('weekly_custom берёт только выбранные дни и не уходит раньше базовой даты', () => {
  // 1 сентября 2026 — вторник; просим Пн и Ср.
  const dates = generateDates(TUE_1_SEP, { type: 'weekly_custom', days: [1, 3], count: 4 })

  assert.deepEqual(dates.map((d) => new Date(d).getDay()), [3, 1, 3, 1])
  assert.ok(new Date(dates[0]) >= new Date(TUE_1_SEP))
})

test('урок в последний день курса попадает в серию', () => {
  const base = new Date('2026-12-24T17:00:00+05:00').toISOString()
  const dates = generateDates(base, { type: 'weekly_same' }, '2026-12-31T00:00:00Z')

  assert.equal(dates.length, 2)
  assert.equal(new Date(dates[1]).getDate(), 31)
})

test('воскресенье — седьмой день, а не нулевой', () => {
  assert.equal(isoWeekday(new Date('2026-09-01T17:00:00+05:00')), 2)  // вторник
  assert.equal(isoWeekday(new Date('2026-09-06T17:00:00+05:00')), 7)  // воскресенье
  assert.equal(isoWeekday(new Date('2026-09-07T17:00:00+05:00')), 1)  // понедельник
})

test('склонение уроков', () => {
  assert.equal(lessonsPlural(1), '1 урок')
  assert.equal(lessonsPlural(2), '2 урока')
  assert.equal(lessonsPlural(53), '53 урока')
  assert.equal(lessonsPlural(11), '11 уроков')
})
