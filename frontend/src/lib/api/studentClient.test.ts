import { test } from 'node:test'
import assert from 'node:assert/strict'
import { shouldRefreshOn401 } from './studentClient.ts'

test('аноним (гость пробного урока) не рефрешится и не выкидывается на логин', () => {
  assert.equal(shouldRefreshOn401('/student/me', false), false)
})

test('ученик с токеном: протухший access — рефрешим', () => {
  assert.equal(shouldRefreshOn401('/student/me', true), true)
})

test('401 по делу (неверный пароль, авторизация) — отдаём форме как есть', () => {
  assert.equal(shouldRefreshOn401('/student/password', true), false)
  assert.equal(shouldRefreshOn401('/student/auth/login', true), false)
})
