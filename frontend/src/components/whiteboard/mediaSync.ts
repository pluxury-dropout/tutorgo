export type MediaAction = 'open' | 'play' | 'pause' | 'seek' | 'close' | 'req'

export interface MediaPayload {
  action: MediaAction
  /** id embeddable-элемента: кадр про YouTube-ролик на доске. Без id кадр
   *  относится к плееру материалов (он на доске один). */
  id?: string
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

/** videoId из любой формы ссылки, либо null. Принимаем и голый id — препод
 *  копирует ссылку на ходу, разбираться с форматом ему некогда. */
export function parseYouTubeId(input: string): string | null {
  const s = input.trim()
  if (/^[\w-]{11}$/.test(s)) return s
  try {
    const u = new URL(s)
    const host = u.hostname.replace(/^www\./, '')
    if (host === 'youtu.be') {
      const id = u.pathname.slice(1)
      return /^[\w-]{11}$/.test(id) ? id : null
    }
    if (host === 'youtube.com' || host === 'm.youtube.com' || host === 'youtube-nocookie.com') {
      const v = u.searchParams.get('v')
      if (v && /^[\w-]{11}$/.test(v)) return v
      // /embed/<id>, /shorts/<id>, /live/<id>
      const m = u.pathname.match(/^\/(?:embed|shorts|live|v)\/([\w-]{11})/)
      if (m) return m[1]
    }
  } catch {
    // не URL — значит и не ссылка на YouTube
  }
  return null
}
