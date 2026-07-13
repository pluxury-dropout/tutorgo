import { test } from 'node:test'
import assert from 'node:assert/strict'
import { nextMediaState, isPlayable } from './mediaSync.ts'

test('open вводит новое состояние', () => {
  const s = nextMediaState(null, {
    action: 'open',
    url: 'https://s3/u.mp3',
    mimeType: 'audio/mpeg',
    name: 'u.mp3',
  })
  assert.deepEqual(s, {
    url: 'https://s3/u.mp3',
    mimeType: 'audio/mpeg',
    name: 'u.mp3',
  })
})

test('close сбрасывает состояние', () => {
  const open = { url: 'u', mimeType: 'audio/mpeg', name: 'n' }
  assert.equal(nextMediaState(open, { action: 'close' }), null)
})

test('play/pause/seek не меняют открытый файл', () => {
  const open = { url: 'u', mimeType: 'audio/mpeg', name: 'n' }
  for (const action of ['play', 'pause', 'seek'] as const) {
    assert.deepEqual(nextMediaState(open, { action, position: 4 }), open)
  }
})

test('open без url игнорируется — не сносим играющий файл битым кадром', () => {
  const open = { url: 'u', mimeType: 'audio/mpeg', name: 'n' }
  assert.deepEqual(nextMediaState(open, { action: 'open' }), open)
})

test('req не меняет состояние', () => {
  const open = { url: 'u', mimeType: 'audio/mpeg', name: 'n' }
  assert.deepEqual(nextMediaState(open, { action: 'req' }), open)
})

test('isPlayable: только аудио и видео', () => {
  assert.equal(isPlayable('audio/mpeg'), true)
  assert.equal(isPlayable('video/mp4'), true)
  assert.equal(isPlayable('application/pdf'), false)
  assert.equal(isPlayable(''), false)
})
