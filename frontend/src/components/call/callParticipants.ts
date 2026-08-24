// Чистая логика панели участников (без React/LiveKit) — тестируется node:test.

const AVATAR_COLORS = ['#5865F2', '#EB459E', '#3BA55D', '#F2924B', '#5A9BD5']

/**
 * Ключ человека из LiveKit identity ("tutor-<uuid>" | "student-<uuid>" |
 * "guest-<ts>"). Единственный общий ключ между участником звонка и
 * коллаборатором доски (Excalidraw кладёт его же в collaborator.id,
 * см. useBoardDisplayName), поэтому по нему и включается follow.
 *
 * У гостя пробного урока аккаунта нет, и ключом служит хвост его же identity:
 * CallRoom кладёт ровно его в BoardIdentity.uid, так что обе стороны считают
 * ключ из одной строки. Мусор без известного префикса → null, такой участник
 * не кликабелен.
 */
export function uidOf(identity: string): string | null {
  const i = identity.indexOf('-')
  if (i < 0) return null
  const prefix = identity.slice(0, i)
  if (prefix !== 'tutor' && prefix !== 'student' && prefix !== 'guest') return null
  return identity.slice(i + 1) || null
}

/** Инициалы для аватара: «Иван Петров» → «ИП», «Репетитор» → «РЕ». */
export function initialsOf(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean)
  if (words.length === 0) return '?'
  if (words.length > 1) return (words[0][0] + words[1][0]).toUpperCase()
  return words[0].slice(0, 2).toUpperCase()
}

/**
 * uid → имя из коллабораторов доски. Настоящее имя человека живёт только здесь:
 * пиры шлют его в cursor-сообщениях (см. useExcalidrawSync), а LiveKit знает
 * лишь роль («Репетитор», «Ученик»). Себя пропускаем — своё имя берётся из
 * BoardIdentity; коллаборатора без id (аноним по ссылке) сопоставить не с чем.
 */
export function peerNames(
  collaborators: ReadonlyMap<string, { id?: string; username?: string | null; isCurrentUser?: boolean }>
): Record<string, string> {
  const out: Record<string, string> = {}
  for (const c of collaborators.values()) {
    if (c.isCurrentUser || !c.id || !c.username) continue
    out[c.id] = c.username
  }
  return out
}

/** Поверхностное сравнение — гасит ререндеры на потоке onChange от рисования. */
export function shallowEqual(
  a: Record<string, string>,
  b: Record<string, string>
): boolean {
  const ak = Object.keys(a)
  if (ak.length !== Object.keys(b).length) return false
  return ak.every((k) => a[k] === b[k])
}

/** Детерминированный цвет аватара — один и тот же участник не «мигает» цветом. */
export function colorOf(identity: string): string {
  let hash = 0
  for (const ch of identity) hash = (hash * 31 + ch.charCodeAt(0)) >>> 0
  return AVATAR_COLORS[hash % AVATAR_COLORS.length]
}
