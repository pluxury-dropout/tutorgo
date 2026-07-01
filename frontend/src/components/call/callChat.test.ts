import { test } from 'node:test'
import assert from 'node:assert/strict'
import { encodeChat, parseChatMessage } from './callChat.ts'

test('encode → parse round-trip', () => {
  const bytes = encodeChat('Мария П.', 'Привет')
  assert.deepEqual(parseChatMessage(bytes), { from: 'Мария П.', text: 'Привет' })
})

test('parseChatMessage игнорирует не-chat сообщения (board-open)', () => {
  const board = new TextEncoder().encode(
    JSON.stringify({ type: 'board-open', board_token: 'abc' }),
  )
  assert.equal(parseChatMessage(board), null)
})

test('parseChatMessage возвращает null на битом payload', () => {
  assert.equal(parseChatMessage(new TextEncoder().encode('not json')), null)
})

test('encodeChat помечает сообщение type:chat', () => {
  const decoded = JSON.parse(new TextDecoder().decode(encodeChat('X', 'y')))
  assert.equal(decoded.type, 'chat')
})
