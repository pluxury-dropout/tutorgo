export type MediaAction = 'open' | 'play' | 'pause' | 'seek' | 'close' | 'req'

export interface MediaPayload {
  action: MediaAction
  url?: string
  mimeType?: string
  name?: string
  /** Позиция в секундах. */
  position?: number
}

export interface MediaState {
  url: string
  mimeType: string
  name: string
}

/** Какой файл открыт после применения кадра. Транспортные действия
 *  (play/pause/seek/req) файл не меняют — они правят только сам элемент. */
export function nextMediaState(
  prev: MediaState | null,
  p: MediaPayload
): MediaState | null {
  if (p.action === 'close') return null
  if (p.action === 'open') {
    // Битый open (без url) игнорируем, иначе он снесёт играющий файл.
    if (!p.url || !p.mimeType) return prev
    return { url: p.url, mimeType: p.mimeType, name: p.name ?? '' }
  }
  return prev
}

export function isPlayable(mimeType: string): boolean {
  return mimeType.startsWith('audio/') || mimeType.startsWith('video/')
}
