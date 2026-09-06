import { test } from 'node:test'
import assert from 'node:assert/strict'
import { patchEntry, feedRange, inRange } from './calendarPatch.ts'

const at = (h: number) => `2026-09-07T0${h}:00:00.000Z`

const feedEvent = () => ({
  id: 'e1', type: 'event', title: 'Спортзал',
  starts_at: at(7), duration_minutes: 90,
  event: { id: 'e1', title: 'Спортзал', kind: 'personal', starts_at: at(7), duration_minutes: 90, color: '#fff' },
})

test('лента: время правится и наверху, и во вложенном объекте', () => {
  const out = patchEntry(feedEvent(), { starts_at: at(9), duration_minutes: 60 })
  assert.equal(out.starts_at, at(9))
  assert.equal(out.duration_minutes, 60)
  const nested = out.event as Record<string, unknown>
  assert.equal(nested.starts_at, at(9))
  assert.equal(nested.duration_minutes, 60)
  // поповер события читает вложенный объект — остальные его поля не теряем
  assert.equal(nested.color, '#fff')
})

test('список уроков без вложенного объекта: пишется scheduled_at', () => {
  const lesson = { id: 'l1', scheduled_at: at(8), duration_minutes: 60, status: 'scheduled' }
  const out = patchEntry(lesson, { starts_at: at(9), duration_minutes: 45 })
  assert.equal(out.scheduled_at, at(9))
  assert.equal(out.duration_minutes, 45)
  assert.equal(out.status, 'scheduled')
})

test('extra ложится только во вложенный объект', () => {
  const out = patchEntry(feedEvent(), { starts_at: at(9), duration_minutes: 60 }, { notes: 'взять форму' })
  assert.equal(out.notes, undefined)
  assert.equal((out.event as Record<string, unknown>).notes, 'взять форму')
})

test('undefined в extra не затирает поле — notes у урока остаются', () => {
  const lesson = {
    id: 'l1', type: 'lesson', title: 'Алгебра',
    starts_at: at(8), duration_minutes: 60,
    lesson: { id: 'l1', scheduled_at: at(8), duration_minutes: 60, status: 'scheduled', notes: 'принести тетрадь' },
  }
  const out = patchEntry(lesson, { starts_at: at(9), duration_minutes: 60 }, { status: 'scheduled', notes: undefined })
  assert.equal((out.lesson as Record<string, unknown>).notes, 'принести тетрадь')
})

test('title меняется, только когда его передали', () => {
  const keep = patchEntry(feedEvent(), { starts_at: at(9), duration_minutes: 60 })
  assert.equal(keep.title, 'Спортзал')
  const renamed = patchEntry(feedEvent(), { starts_at: at(9), duration_minutes: 60, title: 'Бассейн' })
  assert.equal(renamed.title, 'Бассейн')
})

test('диапазон берём только у ленты — список уроков и конфликты вставку не принимают', () => {
  const week: [string, string] = ['2026-09-07T00:00:00.000Z', '2026-09-14T00:00:00.000Z']
  assert.deepEqual(feedRange(['calendar', 'feed', ...week]), week)
  assert.equal(feedRange(['calendar', ...week]), null)                      // список уроков
  assert.equal(feedRange(['calendar', 'conflicts', { starts_at: at(9) }]), null)
  assert.equal(feedRange(['tasks', 'board']), null)
})

test('в чужую неделю оптимистичная запись не попадает', () => {
  const week: [string, string] = ['2026-09-07T00:00:00.000Z', '2026-09-14T00:00:00.000Z']
  assert.ok(inRange('2026-09-07T00:00:00.000Z', week))  // начало включительно
  assert.ok(inRange('2026-09-10T12:00:00.000Z', week))
  assert.ok(!inRange('2026-09-14T00:00:00.000Z', week)) // конец исключительно — это уже следующая
  assert.ok(!inRange('2026-09-06T23:59:00.000Z', week))
})

test('исходная запись не мутируется — откат должен вернуть прежнее', () => {
  const entry = feedEvent()
  patchEntry(entry, { starts_at: at(9), duration_minutes: 60 })
  assert.equal(entry.starts_at, at(7))
  assert.equal(entry.event.starts_at, at(7))
})
