import { test } from 'node:test'
import assert from 'node:assert/strict'
import { effectiveStatus } from './lessonStatus.ts'

const iso = (msFromNow: number) => new Date(Date.now() + msFromNow).toISOString()

test('scheduled + время окончания прошло → completed', () => {
  assert.equal(effectiveStatus({ status: 'scheduled', scheduled_at: iso(-90 * 60_000), duration_minutes: 60 }), 'completed')
})

test('scheduled, урок ещё идёт → scheduled', () => {
  assert.equal(effectiveStatus({ status: 'scheduled', scheduled_at: iso(-10 * 60_000), duration_minutes: 60 }), 'scheduled')
})

test('уже cancelled не переопределяется временем', () => {
  assert.equal(effectiveStatus({ status: 'cancelled', scheduled_at: iso(-90 * 60_000), duration_minutes: 60 }), 'cancelled')
})
