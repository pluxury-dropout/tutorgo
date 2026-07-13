import { test } from 'node:test'
import assert from 'node:assert/strict'
import { uidOf, initialsOf, colorOf, peerNames, shallowEqual } from './callParticipants.ts'

test('uidOf: uuid тьютора и ученика — ключ склейки со списком доски', () => {
  const uuid = '3f2b1c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d'
  assert.equal(uidOf(`tutor-${uuid}`), uuid)
  assert.equal(uidOf(`student-${uuid}`), uuid)
})

test('uidOf: аноним по ссылке и мусор → null (follow недоступен)', () => {
  assert.equal(uidOf('guest-1783951586596'), null)
  assert.equal(uidOf('tutor-'), null)
  assert.equal(uidOf('bare'), null)
})

test('initialsOf: два слова → первые буквы, одно → две буквы', () => {
  assert.equal(initialsOf('Иван Петров'), 'ИП')
  assert.equal(initialsOf('Репетитор'), 'РЕ')
  assert.equal(initialsOf('  '), '?')
})

test('colorOf: детерминирован — цвет аватара не мигает между рендерами', () => {
  assert.equal(colorOf('tutor-abc'), colorOf('tutor-abc'))
})

test('peerNames: имя человека берётся с доски, себя и анонима пропускаем', () => {
  const collaborators = new Map([
    ['self', { id: 'me', username: 'Я Сам', isCurrentUser: true }],
    ['p1', { id: 'uid-1', username: 'Иван Петров' }],
    ['p2', { username: 'Гость' }], // аноним по ссылке: без id сопоставить не с чем
  ])
  assert.deepEqual(peerNames(collaborators), { 'uid-1': 'Иван Петров' })
})

test('shallowEqual: одинаковые карты имён не вызывают ререндер', () => {
  assert.equal(shallowEqual({ a: 'Иван' }, { a: 'Иван' }), true)
  assert.equal(shallowEqual({ a: 'Иван' }, { a: 'Пётр' }), false)
  assert.equal(shallowEqual({ a: 'Иван' }, { a: 'Иван', b: 'Пётр' }), false)
})
