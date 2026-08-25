import { test } from 'node:test'
import assert from 'node:assert/strict'
import { cycleProgress } from './cycleProgress.ts'
import type { CalendarLesson } from '@/types/api'

const NOW = Date.parse('2026-08-12T12:00:00Z')

// Уроки различаются только тем, что важно для цикла; остальное — заглушки.
function lesson(p: Partial<CalendarLesson> & { scheduled_at: string }): CalendarLesson {
  return {
    id: p.scheduled_at,
    course_id: 'c1',
    duration_minutes: 60,
    status: 'scheduled',
    notes: '',
    subject: 'Алгебра',
    student_name: null,
    is_group: false,
    ...p,
  }
}

test('позади position-1 уроков, когда опорный — ближайший будущий', () => {
  const got = cycleProgress(
    [
      lesson({ scheduled_at: '2026-08-05T10:00:00Z', cycle_position: 4, cycle_size: 8 }),
      lesson({ scheduled_at: '2026-08-14T10:00:00Z', cycle_position: 5, cycle_size: 8 }),
      lesson({ scheduled_at: '2026-08-21T10:00:00Z', cycle_position: 6, cycle_size: 8 }),
    ],
    NOW,
  )
  assert.deepEqual(got, [{ courseId: 'c1', subject: 'Алгебра', done: 4, size: 8 }])
})

test('идущий урок ещё не позади: опорным остаётся он, а не прошлый', () => {
  // Начался 30 минут назад, длится 60 → закончится через 30.
  const got = cycleProgress(
    [
      lesson({ scheduled_at: '2026-08-05T10:00:00Z', cycle_position: 4, cycle_size: 8 }),
      lesson({ scheduled_at: '2026-08-12T11:30:00Z', cycle_position: 5, cycle_size: 8 }),
    ],
    NOW,
  )
  assert.equal(got[0].done, 4)
})

test('будущих уроков нет — считаем по последнему прошедшему', () => {
  const got = cycleProgress(
    [
      lesson({ scheduled_at: '2026-08-01T10:00:00Z', cycle_position: 7, cycle_size: 8 }),
      lesson({ scheduled_at: '2026-08-08T10:00:00Z', cycle_position: 8, cycle_size: 8 }),
    ],
    NOW,
  )
  assert.deepEqual(got, [{ courseId: 'c1', subject: 'Алгебра', done: 8, size: 8 }])
})

test('курсы считаются раздельно и отдаются в алфавитном порядке', () => {
  const got = cycleProgress(
    [
      lesson({ scheduled_at: '2026-08-14T10:00:00Z', cycle_position: 5, cycle_size: 8 }),
      lesson({
        scheduled_at: '2026-08-15T10:00:00Z',
        course_id: 'c2',
        subject: 'Английский',
        cycle_position: 2,
        cycle_size: 4,
      }),
    ],
    NOW,
  )
  assert.deepEqual(got, [
    { courseId: 'c1', subject: 'Алгебра', done: 4, size: 8 },
    { courseId: 'c2', subject: 'Английский', done: 1, size: 4 },
  ])
})

test('уроки вне оплаченного цикла не дают строку прогресса', () => {
  const got = cycleProgress([lesson({ scheduled_at: '2026-08-14T10:00:00Z' })], NOW)
  assert.deepEqual(got, [])
})

test('порядок во входном массиве не влияет на результат', () => {
  const items = [
    lesson({ scheduled_at: '2026-08-21T10:00:00Z', cycle_position: 6, cycle_size: 8 }),
    lesson({ scheduled_at: '2026-08-05T10:00:00Z', cycle_position: 4, cycle_size: 8 }),
    lesson({ scheduled_at: '2026-08-14T10:00:00Z', cycle_position: 5, cycle_size: 8 }),
  ]
  assert.deepEqual(cycleProgress(items, NOW), cycleProgress([...items].reverse(), NOW))
})
