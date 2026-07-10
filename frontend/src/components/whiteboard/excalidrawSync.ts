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

// FileReader — браузерный API, в node:test не гоняется (и не нужно).
export function blobToDataURL(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string)
    reader.onerror = () => reject(reader.error)
    reader.readAsDataURL(blob)
  })
}
