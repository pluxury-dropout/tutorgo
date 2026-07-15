import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  nextMediaState,
  isPlayable,
  parseYouTubeId,
  isEchoOfRemote,
  isSeekEcho,
} from './mediaSync.ts'

test('плеер подтверждает применённую чужую команду — это эхо, рассылать нельзя', () => {
  assert.equal(isEchoOfRemote(true, 'play'), true)
  assert.equal(isEchoOfRemote(false, 'pause'), true)
})

test('плеер сменил состояние вопреки согласованному — это живой человек', () => {
  assert.equal(isEchoOfRemote(true, 'pause'), false)
  assert.equal(isEchoOfRemote(false, 'play'), false)
})

test('первое действие с роликом (согласованного состояния ещё нет) — не эхо', () => {
  assert.equal(isEchoOfRemote(undefined, 'play'), false)
  assert.equal(isEchoOfRemote(undefined, 'pause'), false)
})

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

test('isSeekEcho: попали в выставленную нами позицию — это эхо', () => {
  assert.equal(isSeekEcho(30, 30.1), true)
  assert.equal(isSeekEcho(30, 30), true)
})

test('isSeekEcho: живая перемотка далеко от нашей метки — не эхо', () => {
  assert.equal(isSeekEcho(30, 42), false)
  assert.equal(isSeekEcho(null, 30), false)
})

test('isPlayable: только аудио и видео', () => {
  assert.equal(isPlayable('audio/mpeg'), true)
  assert.equal(isPlayable('video/mp4'), true)
  assert.equal(isPlayable('application/pdf'), false)
  assert.equal(isPlayable(''), false)
})

test('parseYouTubeId: рабочие формы ссылки', () => {
  const id = 'dQw4w9WgXcQ'
  assert.equal(parseYouTubeId(`https://www.youtube.com/watch?v=${id}`), id)
  assert.equal(parseYouTubeId(`https://youtube.com/watch?v=${id}&t=42s`), id)
  assert.equal(parseYouTubeId(`https://youtu.be/${id}?si=abc`), id)
  assert.equal(parseYouTubeId(`https://www.youtube.com/embed/${id}`), id)
  assert.equal(parseYouTubeId(`https://www.youtube.com/shorts/${id}`), id)
  assert.equal(parseYouTubeId(`  ${id}  `), id)
})

test('parseYouTubeId: мусор отвергнут', () => {
  assert.equal(parseYouTubeId('https://vimeo.com/12345'), null)
  assert.equal(parseYouTubeId('https://www.youtube.com/watch?v=short'), null)
  assert.equal(parseYouTubeId('просто текст'), null)
  assert.equal(parseYouTubeId(''), null)
})
