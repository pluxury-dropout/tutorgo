import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  diffChangedElements,
  markRemoteVersions,
  parseSnapshot,
  utf8ByteSize,
  mergeCollaborators,
  imageFromClipboard,
  serializeSnapshot,
  serializeUpdate,
  compactTombstones,
} from './excalidrawSync.ts'

test('imageFromClipboard: возвращает первый image-файл', () => {
  const png = new File([new Uint8Array([1])], 'a.png', { type: 'image/png' })
  const dt = { files: [png] } as unknown as DataTransfer
  assert.equal(imageFromClipboard(dt), png)
})

test('imageFromClipboard: не-картинка и пустой буфер → null', () => {
  const txt = new File(['x'], 'a.txt', { type: 'text/plain' })
  assert.equal(imageFromClipboard({ files: [txt] } as unknown as DataTransfer), null)
  assert.equal(imageFromClipboard({ files: [] } as unknown as DataTransfer), null)
  assert.equal(imageFromClipboard(null), null)
})

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

test('compactTombstones: выбрасывает только протухшие удалённые', () => {
  const now = 1_700_000_000_000
  const day = 24 * 60 * 60 * 1000
  const got = compactTombstones(
    [
      { id: 'живой', updated: now - 10 * day },
      { id: 'живой-без-updated' },
      { id: 'свежий-труп', isDeleted: true, updated: now - day / 2 },
      { id: 'старый-труп', isDeleted: true, updated: now - 2 * day },
      // Возраст неизвестен — не наша забота его хоронить.
      { id: 'труп-без-updated', isDeleted: true },
    ],
    now
  )
  assert.deepEqual(
    got.map((el) => el.id),
    ['живой', 'живой-без-updated', 'свежий-труп', 'труп-без-updated']
  )
})

test('serializeSnapshot: компактит протухшие tombstones', () => {
  const now = Date.now()
  const json = serializeSnapshot(
    [
      { id: 'a', updated: now },
      { id: 'b', isDeleted: true, updated: now - 48 * 60 * 60 * 1000 },
    ],
    {}
  )
  const back = JSON.parse(json) as { elements: { id: string }[] }
  assert.deepEqual(
    back.elements.map((el) => el.id),
    ['a']
  )
})

test('serializeSnapshot: режет точность координат, но не ломает данные', () => {
  // Настоящий штрих из БД: каждая точка весила ~40 байт на 15 знаков.
  const points = Array.from({ length: 200 }, (_, i) => [
    i * 0.41971259276760975,
    i * -0.41967416810530267,
  ])
  const el = {
    id: 'vdeKTFM8OrFzA9xCKCnnk',
    type: 'freedraw',
    x: 6567.954082645468,
    y: 1991.2004047360947,
    seed: 1847884469,
    version: 42,
    angle: 1.5707963267948966,
    isDeleted: false,
    points,
  }
  const json = serializeSnapshot([el], {})
  const back = JSON.parse(json) as { elements: (typeof el)[] }
  const got = back.elements[0]

  // Идентичность элемента и версионирование не тронуты — на них держится reconcile.
  assert.equal(got.id, el.id)
  assert.equal(got.seed, el.seed)
  assert.equal(got.version, el.version)
  // Геометрия округлена до сотой доли пикселя.
  assert.equal(got.x, 6567.95)
  assert.equal(got.points.length, 200)
  assert.equal(got.points[1][0], 0.42)
  // angle округляется мягче — двух знаков хватило бы на перекос в 0.3°.
  assert.equal(got.angle, 1.5708)

  // Ради чего всё: тот же штрих раньше не влезал в лимит хаба. На реальных
  // досках выигрыш выше (~3.5×) — тут координаты короче настоящих.
  const naive = JSON.stringify({ elements: [el], files: {} })
  assert.ok(
    utf8ByteSize(json) * 2 < utf8ByteSize(naive),
    `ожидали сжатие вдвое, вышло ${utf8ByteSize(naive)} → ${utf8ByteSize(json)}`
  )
})

test('serializeUpdate: готовое тело WS-сообщения с округлёнными координатами', () => {
  // Тот же штрих, что и в тесте serializeSnapshot: flushUpdate раньше слал
  // его голым JSON.stringify — координаты уезжали полными float64.
  const points = Array.from({ length: 200 }, (_, i) => [
    i * 0.41971259276760975,
    i * -0.41967416810530267,
  ])
  const el = {
    id: 'vdeKTFM8OrFzA9xCKCnnk',
    type: 'freedraw',
    x: 6567.954082645468,
    y: 1991.2004047360947,
    seed: 1847884469,
    version: 42,
    versionNonce: 918273645,
    angle: 1.5707963267948966,
    isDeleted: false,
    points,
  }
  const json = serializeUpdate([el])
  const parsed = JSON.parse(json) as {
    type: string
    payload: { elements: (typeof el)[] }
  }

  // Валидный конверт WS-сообщения — формат протокола не меняется.
  assert.equal(parsed.type, 'update')
  assert.ok(Array.isArray(parsed.payload.elements))
  const got = parsed.payload.elements[0]

  // Координаты (x/y и точки freedraw) округлены до COORD_PRECISION.
  assert.equal(got.x, 6567.95)
  assert.equal(got.points[1][0], 0.42)
  assert.equal(got.points[1][1], -0.42)
  // angle — до ANGLE_PRECISION.
  assert.equal(got.angle, 1.5708)

  // Ради чего всё: тот же штрих раньше не влезал в округление вовсе — этот
  // путь сериализации был голым JSON.stringify.
  const naive = JSON.stringify({ type: 'update', payload: { elements: [el] } })
  assert.ok(
    utf8ByteSize(json) * 2 < utf8ByteSize(naive),
    `ожидали сжатие вдвое, вышло ${utf8ByteSize(naive)} → ${utf8ByteSize(json)}`
  )
})

// Ключевой тест: правило слияния Excalidraw держится на version/versionNonce/
// seed/id. Это целые числа — округление их не меняет по конструкции
// roundFloats (Math.round на целом — тот же целый), но именно этот факт и
// нужно зафиксировать явно: искажение любого из них ломает reconcile на
// клиенте и UPSERT на сервере молча, доска у двоих разъезжается без единой
// ошибки в логах.
test('serializeUpdate: version/versionNonce/seed/id проходят без изменений', () => {
  const el = {
    id: 'vdeKTFM8OrFzA9xCKCnnk',
    type: 'freedraw',
    x: 1.23456,
    y: -9.87654,
    seed: 1847884469,
    version: 42,
    versionNonce: 918273645,
    angle: 0.12345678,
    points: [[0.123456, 0.654321]],
  }
  const json = serializeUpdate([el])
  const got = (
    JSON.parse(json) as { payload: { elements: (typeof el)[] } }
  ).payload.elements[0]

  assert.equal(got.id, el.id)
  assert.equal(got.version, el.version)
  assert.equal(got.versionNonce, el.versionNonce)
  assert.equal(got.seed, el.seed)
  // Геометрия при этом реально округлена — не совпадает с исходной.
  assert.notEqual(got.x, el.x)
  assert.notEqual(got.angle, el.angle)
})

test('serializeUpdate: tombstone (isDeleted) не выбрасывается, в отличие от serializeSnapshot', () => {
  const tombstone = {
    id: 'dead-el',
    version: 5,
    isDeleted: true,
    updated: Date.now() - 48 * 60 * 60 * 1000, // протух бы в compactTombstones
    x: 1,
    y: 1,
  }
  const json = serializeUpdate([tombstone])
  const got = (
    JSON.parse(json) as { payload: { elements: { id: string }[] } }
  ).payload.elements
  assert.deepEqual(got.map((el) => el.id), ['dead-el'])
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

// Регрессия: страницы PDF пропадали у пира. applyRemote затирал всю карту
// версий сценой, и локальная страница, ещё не улетевшая троттлом flushUpdate,
// оказывалась «уже отправленной».
test('markRemoteVersions: локальный элемент, не успевший уехать, остаётся в диффе', () => {
  const versions = new Map<string, number>()
  // Пир прислал свою правку, пока страница PDF ждёт flushUpdate.
  markRemoteVersions(versions, [{ id: 'peer-stroke', version: 7 }])
  const scene = [
    { id: 'peer-stroke', version: 7 },
    { id: 'pdf-page-10', version: 1 },
  ]
  const { changed } = diffChangedElements(versions, scene)
  assert.deepEqual(
    changed.map((el) => el.id),
    ['pdf-page-10']
  )
})

test('markRemoteVersions: победивший локальный элемент уезжает пиру', () => {
  const versions = new Map<string, number>([['el', 3]])
  markRemoteVersions(versions, [{ id: 'el', version: 4 }])
  // reconcile оставил локальную версию 5 — пир о ней ещё не знает.
  const { changed } = diffChangedElements(versions, [{ id: 'el', version: 5 }])
  assert.deepEqual(changed, [{ id: 'el', version: 5 }])
})
