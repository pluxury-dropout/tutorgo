import { test } from 'node:test'
import assert from 'node:assert/strict'
import { shouldToastQueryError } from './queryError.ts'

test('401 и 402 не тостим — их ведёт перехватчик', () => {
  assert.equal(shouldToastQueryError({ message: 'Unauthorized', status: 401 }), false)
  assert.equal(shouldToastQueryError({ message: 'subscription_required', status: 402 }), false)
})

test('500 тостим — это и был немой сбой', () => {
  assert.equal(shouldToastQueryError({ message: 'internal server error', status: 500 }), true)
})

test('прочие статусы тостим', () => {
  assert.equal(shouldToastQueryError({ message: 'not found', status: 404 }), true)
  assert.equal(shouldToastQueryError({ message: 'conflict', status: 409 }), true)
})

test('ошибка без статуса (обрыв сети, не-ApiError) тостится', () => {
  assert.equal(shouldToastQueryError({ message: 'Network Error', status: 0 }), true)
  assert.equal(shouldToastQueryError(new Error('boom')), true)
  assert.equal(shouldToastQueryError(null), true)
  assert.equal(shouldToastQueryError(undefined), true)
})

test('статус строкой не считается за 401 — сравнение строгое', () => {
  assert.equal(shouldToastQueryError({ status: '401' }), true)
})
