import { test } from 'node:test'
import assert from 'node:assert/strict'
import { pickActiveLesson } from './nextLesson.ts'
import type { CalendarLesson } from '@/types/api'

const NOW = new Date('2026-07-14T10:00:00Z')

function lesson(over: Partial<CalendarLesson>): CalendarLesson {
  return {
    id: 'l1',
    course_id: 'c1',
    scheduled_at: '2026-07-14T10:05:00Z',
    duration_minutes: 60,
    status: 'scheduled',
    notes: '',
    subject: 'Английский',
    student_name: 'Аня',
    is_group: false,
    ...over,
  }
}

test('урок через 5 минут — попадает в окно', () => {
  const l = lesson({})
  assert.equal(pickActiveLesson([l], NOW)?.id, 'l1')
})

test('урок через 11 минут — окна нет', () => {
  const l = lesson({ scheduled_at: '2026-07-14T10:11:00Z' })
  assert.equal(pickActiveLesson([l], NOW), null)
})

test('ровно −10 минут — граница включительно', () => {
  const l = lesson({ scheduled_at: '2026-07-14T10:10:00Z' })
  assert.equal(pickActiveLesson([l], NOW)?.id, 'l1')
})

test('урок уже идёт — всё ещё в окне', () => {
  const l = lesson({ scheduled_at: '2026-07-14T09:30:00Z', duration_minutes: 60 })
  assert.equal(pickActiveLesson([l], NOW)?.id, 'l1')
})

test('урок кончился минуту назад — окна нет', () => {
  const l = lesson({ scheduled_at: '2026-07-14T08:59:00Z', duration_minutes: 60 })
  assert.equal(pickActiveLesson([l], NOW), null)
})

test('не-scheduled игнорируются', () => {
  const cancelled = lesson({ status: 'cancelled' })
  const completed = lesson({ id: 'l2', status: 'completed' })
  assert.equal(pickActiveLesson([cancelled, completed], NOW), null)
})

test('из двух подходящих берётся ближайший по времени', () => {
  const later  = lesson({ id: 'late',  scheduled_at: '2026-07-14T10:09:00Z' })
  const sooner = lesson({ id: 'soon',  scheduled_at: '2026-07-14T10:02:00Z' })
  assert.equal(pickActiveLesson([later, sooner], NOW)?.id, 'soon')
})

test('пустой список — null', () => {
  assert.equal(pickActiveLesson([], NOW), null)
})
