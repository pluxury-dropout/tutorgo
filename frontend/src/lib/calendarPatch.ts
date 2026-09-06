// Чистая часть оптимистичного переноса: как выглядит запись календаря после
// сдвига. Живёт отдельно от хука, чтобы гоняться через node --test — хук тянет
// axios и алиасы, которых у node нет.

/** Запись под ключом ['calendar'] — урок из списка или элемент ленты. Точный
 *  тип не нужен: патч трогает общие поля и вложенный объект по имени типа. */
export type CalendarEntry = Record<string, unknown> & { id?: string; type?: string }

export type TimePatch = { starts_at: string; duration_minutes: number; title?: string }

/** Сдвинутая копия записи.
 *
 *  Под ключом ['calendar'] лежат две формы: список уроков (`scheduled_at`) и
 *  лента (`starts_at` плюс вложенный объект по типу). Пишем оба поля и на
 *  верхнем уровне, и во вложенном — лишнее просто не читается, а вот
 *  недописанное вылезает: поповер покажет старое время, а второй перенос
 *  подряд отсчитается от старого слота.
 *
 *  `extra` домешивается только во вложенный объект — статус урока, поля
 *  события: наверху ленты их нет. */
export function patchEntry(
  entry: CalendarEntry,
  patch: TimePatch,
  extra?: Record<string, unknown>,
): CalendarEntry {
  const time = {
    starts_at:        patch.starts_at,
    scheduled_at:     patch.starts_at,
    duration_minutes: patch.duration_minutes,
  }
  const nestedKey = entry.type
  const nested    = nestedKey ? (entry[nestedKey] as CalendarEntry | undefined) : undefined

  return {
    ...entry,
    ...time,
    ...(patch.title !== undefined ? { title: patch.title } : {}),
    ...(nested ? { [nestedKey!]: { ...nested, ...time, ...defined(extra) } } : {}),
  }
}

/** Поля запроса необязательны: `notes: undefined` в spread затёрло бы заметки,
 *  которые сервер сохранит как есть. Пропускаем только реально переданное. */
function defined(o?: Record<string, unknown>): Record<string, unknown> {
  if (!o) return {}
  return Object.fromEntries(Object.entries(o).filter(([, v]) => v !== undefined))
}

/** Диапазон запроса ленты `['calendar', 'feed', from, to]`, иначе null.
 *  Под ключом ['calendar'] лежат ещё список уроков и конфликты — в них новую
 *  запись не вставляют: у них своя форма и свой срок жизни. */
export function feedRange(key: readonly unknown[]): [string, string] | null {
  if (key[0] !== 'calendar' || key[1] !== 'feed') return null
  const [from, to] = [key[2], key[3]]
  return typeof from === 'string' && typeof to === 'string' ? [from, to] : null
}

/** Начало записи внутри диапазона — иначе новый урок мелькнёт в чужой неделе. */
export function inRange(iso: string, [from, to]: [string, string]): boolean {
  const t = Date.parse(iso)
  return t >= Date.parse(from) && t < Date.parse(to)
}
