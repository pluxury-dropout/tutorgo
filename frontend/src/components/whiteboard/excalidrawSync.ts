// Чистые функции синхронизации Excalidraw — вынесены из хука ради node:test.
// Намеренно без импортов из @excalidraw/excalidraw: структурного { id, version }
// достаточно, а node:test не резолвит ESM+CSS этого пакета.

export interface VersionedElement {
  id: string
  version: number
}

// Карта-указатель на файлы картинок: данные живут в S3, здесь только URL.
// Base64 в снапшоты класть нельзя — у Go-хаба ReadLimit 512 КБ на сообщение.
export type SnapshotFiles = Record<string, { url: string; mimeType: string }>

// Размер строки в байтах UTF-8. TextEncoder есть и в браузере, и в node:test
// (Blob не берём — в старых node его не было).
export function utf8ByteSize(s: string): number {
  return new TextEncoder().encode(s).length
}

// Excalidraw держит координаты как полные float64, и в JSON они уезжают со всеми
// знаками: одна точка пера — «[0.41971259276760975, -0.41967416810530267]», 40
// байт, из которых значимы четыре. Штрих на 400 точек весит 17 КБ вместо трёх.
// Режем точность на сериализации; сама сцена не трогается, так что undo, курсоры
// и reconcile работают на полных значениях.
// Два знака — сотая доля пикселя, на порядки ниже порога видимости.
const COORD_PRECISION = 2
// angle — радианы, весь диапазон это 0..2π: два знака дали бы перекос до 0.3°.
// Округляем мягче, элементов с ненулевым angle на доске единицы.
const ANGLE_PRECISION = 4

// Целые (seed, version, versionNonce) округление не меняет — отдельный список
// исключений им не нужен.
function roundFloats(key: string, value: unknown): unknown {
  if (typeof value !== 'number' || !Number.isFinite(value)) return value
  const factor = 10 ** (key === 'angle' ? ANGLE_PRECISION : COORD_PRECISION)
  return Math.round(value * factor) / factor
}

// Свежие tombstones (isDeleted) персистить обязательно: без них пир,
// пропустивший удаление офлайн, воскресит элемент через reconcile. Но копятся
// они вечно — на боевых досках занимали от 34% до 100% веса снапшота. Через
// сутки воскрешать уже некому: вкладка, которая столько провисела с устаревшей
// локальной сценой, до реконнекта не доживает.
const TOMBSTONE_TTL_MS = 24 * 60 * 60 * 1000

interface Perishable {
  isDeleted?: boolean
  updated?: number
}

// now — параметр ради тестируемости, в проде всегда Date.now().
export function compactTombstones<T extends Perishable>(
  elements: readonly T[],
  now: number = Date.now()
): readonly T[] {
  return elements.filter((el) => {
    if (!el.isDeleted) return true
    // Excalidraw бампает `updated` и на удалении, так что для tombstone это
    // момент смерти. Нет поля — возраст неизвестен, такой не трогаем.
    return el.updated === undefined || now - el.updated < TOMBSTONE_TTL_MS
  })
}

// Тело запроса персиста.
export function serializeSnapshot<T extends Perishable>(
  elements: readonly T[],
  files: SnapshotFiles
): string {
  return JSON.stringify(
    { elements: compactTombstones(elements), files },
    roundFloats
  )
}

// Возвращает элементы, чья версия изменилась или которых не было в prev,
// и новую карту версий. Excalidraw бампает version на каждую правку, включая
// удаление (isDeleted: true — tombstone едет как обычный update).
export function diffChangedElements<T extends VersionedElement>(
  prev: ReadonlyMap<string, number>,
  elements: readonly T[]
): { changed: T[]; next: Map<string, number> } {
  const next = new Map<string, number>()
  const changed: T[] = []
  for (const el of elements) {
    next.set(el.id, el.version)
    if (prev.get(el.id) !== el.version) changed.push(el)
  }
  return { changed, next }
}

// Отмечает известными пиру ТОЛЬКО те элементы, что реально приехали от него.
// Затирать всю карту сценой нельзя: локальный элемент, ещё не улетевший
// троттлом flushUpdate, попал бы в «уже отправленные» и не уехал бы к пиру
// никогда — а следом снапшот пира затёр бы его и на сервере.
// Элемент, где локальная версия победила в reconcile, тут получает чужую
// версию → останется в диффе и уедет пиру. Так и надо.
export function markRemoteVersions(
  prev: Map<string, number>,
  remote: readonly VersionedElement[]
): Map<string, number> {
  for (const el of remote) prev.set(el.id, el.version)
  return prev
}

// Снапшот нового формата: { elements: [...], files?: {...} }.
// Старые tldraw-снапшоты ({ document: { store } }) и мусор → null:
// доска стартует с чистого листа (решение из спеки — конвертер не пишем).
export function parseSnapshot(
  payload: unknown
): { elements: VersionedElement[]; files: SnapshotFiles } | null {
  if (typeof payload !== 'object' || payload === null) return null
  const p = payload as { elements?: unknown; files?: unknown }
  if (!Array.isArray(p.elements)) return null
  return {
    elements: p.elements as VersionedElement[],
    files: (p.files as SnapshotFiles | undefined) ?? {},
  }
}

// Список участников доски = удалённые пиры + всегда я сам. Пир, у которого id
// совпал с моим uid, — это моё же второе соединение (вкладка звонка + вкладка
// доски): выкидываем, иначе получаем второго «себя» и призрачный курсор.
// Аноним по ссылке приходит без uid — такие пиры не схлопываются никогда.
export function mergeCollaborators<C extends { id?: string }>(
  peers: ReadonlyMap<string, C>,
  selfKey: string,
  self: C,
  myUid?: string
): Map<string, C> {
  const merged = new Map<string, C>()
  peers.forEach((peer, peerId) => {
    if (myUid && peer.id === myUid) return
    merged.set(peerId, peer)
  })
  merged.set(selfKey, self)
  return merged
}

// FileReader — браузерный API, в node:test не гоняется (и не нужно).
export function blobToDataURL(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string)
    reader.onerror = () => reject(reader.error)
    reader.readAsDataURL(blob)
  })
}

// Первый image-файл из буфера обмена, или null. Используется onPasteCapture, чтобы
// увести вставку картинки в S3-путь мимо отключённой нативки Excalidraw. Текст и
// сериализованные элементы Excalidraw файлов не несут → null → отдаём Excalidraw.
export function imageFromClipboard(dt: DataTransfer | null): File | null {
  if (!dt) return null
  for (const file of Array.from(dt.files)) {
    if (file.type.startsWith('image/')) return file
  }
  return null
}
