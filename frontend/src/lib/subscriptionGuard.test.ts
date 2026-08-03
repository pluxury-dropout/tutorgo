import { test } from 'node:test'
import assert from 'node:assert/strict'
import { decideAccess, stateOnLoadError, PAYWALL_PATH } from './subscriptionGuard.ts'

test('active → render без баннера на любом пути', () => {
  assert.deepEqual(decideAccess('active', '/dashboard'), { action: 'render', banner: false })
  assert.deepEqual(decideAccess('active', PAYWALL_PATH), { action: 'render', banner: false })
})

test('grace → render с баннером на любом пути', () => {
  assert.deepEqual(decideAccess('grace', '/students'), { action: 'render', banner: true })
  assert.deepEqual(decideAccess('grace', PAYWALL_PATH), { action: 'render', banner: true })
})

test('blocked на обычном пути → redirect', () => {
  assert.deepEqual(decideAccess('blocked', '/dashboard'), { action: 'redirect' })
})

test('blocked на самом paywall → render без цикла', () => {
  assert.deepEqual(decideAccess('blocked', PAYWALL_PATH), { action: 'render', banner: false })
})

test('blocked на success-странице → render (дополлить активацию)', () => {
  assert.deepEqual(decideAccess('blocked', '/subscription/success'), { action: 'render', banner: false })
})

test('обрыв связи → null, а не ложный paywall', () => {
  assert.equal(stateOnLoadError(0), null)
})

test('ответ сервера об ошибке → blocked (fail-closed)', () => {
  assert.equal(stateOnLoadError(500), 'blocked')
  assert.equal(stateOnLoadError(403), 'blocked')
  assert.equal(stateOnLoadError(undefined), 'blocked')
})
