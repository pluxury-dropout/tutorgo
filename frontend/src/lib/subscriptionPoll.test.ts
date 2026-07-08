import { test } from 'node:test'
import assert from 'node:assert/strict'
import { pollDecision } from './subscriptionPoll.ts'

test('active → activated (даже при исчерпанных попытках)', () => {
  assert.equal(pollDecision('active', 5), 'activated')
  assert.equal(pollDecision('active', 0), 'activated')
})

test('не-active с оставшимися попытками → wait', () => {
  assert.equal(pollDecision('grace', 3), 'wait')
  assert.equal(pollDecision('blocked', 1), 'wait')
})

test('не-active без попыток → timeout', () => {
  assert.equal(pollDecision('grace', 0), 'timeout')
  assert.equal(pollDecision('blocked', 0), 'timeout')
})
