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

// Потолок снапшота под ReadLimit 512 КБ Go-хаба, с запасом: сообщение больше
// лимита сервер режет → сокет рвётся → реконнект → повторная отправка того же
// снапшота = вечная петля. Лучше пропустить отправку, чем убить сокет.
export const SNAPSHOT_MAX_BYTES = 500 * 1024

// Размер строки в байтах UTF-8. TextEncoder есть и в браузере, и в node:test
// (Blob не берём — в старых node его не было).
export function utf8ByteSize(s: string): number {
  return new TextEncoder().encode(s).length
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
