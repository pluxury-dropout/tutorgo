import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  diffChangedElements,
  parseSnapshot,
  utf8ByteSize,
  mergeCollaborators,
  SNAPSHOT_MAX_BYTES,
} from './excalidrawSync.ts'

test('diffChangedElements: новые и изменённые элементы попадают в changed', () => {
  const prev = new Map([['a', 1], ['b', 2]])
  const els = [
    { id: 'a', version: 1 }, // не изменился
    { id: 'b', version: 3 }, // изменился
    { id: 'c', version: 1 }, // новый
  ]
  const { changed, next } = diffChangedElements(prev, els)
  assert.deepEqual(changed.map((e) => e.id), ['b', 'c'])
  assert.equal(next.get('a'), 1)
  assert.equal(next.get('b'), 3)
  assert.equal(next.get('c'), 1)
})

test('diffChangedElements: пустой prev — все элементы changed', () => {
  const { changed } = diffChangedElements(new Map(), [{ id: 'a', version: 5 }])
  assert.equal(changed.length, 1)
})

test('diffChangedElements: без изменений — changed пуст', () => {
  const prev = new Map([['a', 1]])
  const { changed } = diffChangedElements(prev, [{ id: 'a', version: 1 }])
  assert.equal(changed.length, 0)
})

test('parseSnapshot: новый формат с elements и files', () => {
  const snap = parseSnapshot({
    elements: [{ id: 'a', version: 1 }],
    files: { f1: { url: '/assets/x', mimeType: 'image/png' } },
  })
  assert.ok(snap)
  assert.equal(snap.elements.length, 1)
  assert.equal(snap.files.f1.url, '/assets/x')
})

test('parseSnapshot: files отсутствует — пустая карта', () => {
  const snap = parseSnapshot({ elements: [] })
  assert.ok(snap)
  assert.deepEqual(snap.files, {})
})

test('parseSnapshot: старый tldraw-снапшот → null (чистая доска)', () => {
  assert.equal(parseSnapshot({ document: { store: { 'shape:x': {} } } }), null)
  assert.equal(parseSnapshot(null), null)
  assert.equal(parseSnapshot('garbage'), null)
  assert.equal(parseSnapshot({ elements: 'not-array' }), null)
})

test('utf8ByteSize: ASCII — байт на символ, многобайтовые — больше', () => {
  assert.equal(utf8ByteSize(''), 0)
  assert.equal(utf8ByteSize('abc'), 3)
  // кириллица — 2 байта/символ в UTF-8
  assert.equal(utf8ByteSize('да'), 4)
})

test('size-cap: снапшот под лимитом проходит, сверх — режется', () => {
  const under = 'x'.repeat(SNAPSHOT_MAX_BYTES - 1)
  const over = 'x'.repeat(SNAPSHOT_MAX_BYTES + 1)
  assert.ok(utf8ByteSize(under) <= SNAPSHOT_MAX_BYTES)
  assert.ok(utf8ByteSize(over) > SNAPSHOT_MAX_BYTES)
})

test('mergeCollaborators: своё второе соединение не становится вторым участником', () => {
  const peers = new Map([
    ['sock-1', { username: 'Я', id: 'tutor-1' }], // это я из вкладки звонка
    ['sock-2', { username: 'Ученик', id: 'student-9' }],
  ])
  const merged = mergeCollaborators(
    peers,
    'self',
    { username: 'Я', id: 'tutor-1' },
    'tutor-1'
  )
  assert.deepEqual([...merged.keys()], ['sock-2', 'self'])
})

test('mergeCollaborators: без uid (аноним по ссылке) пиры не схлопываются', () => {
  const peers = new Map<string, { username: string; id?: string }>([
    ['sock-1', { username: 'Гость' }],
    ['sock-2', { username: 'Гость' }],
  ])
  const merged = mergeCollaborators(peers, 'self', { username: 'Вы' }, undefined)
  assert.equal(merged.size, 3)
})
