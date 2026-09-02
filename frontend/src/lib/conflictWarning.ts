import { toast } from 'sonner'

import { calendarApi, type ConflictQuery } from '@/lib/api/calendar'
import { formatTimeRange } from '@/lib/eventKind'

// Перенос уже сохранён — это предупреждение постфактум, без отката: наложение
// бывает осознанным, решает репетитор. Сама проверка необязательна, поэтому
// упавший запрос молчит, а не роняет промис.
export function warnOnConflict(q: ConflictQuery) {
  calendarApi
    .conflicts(q)
    .then((conflicts) => {
      if (conflicts.length === 0) return
      const list = conflicts
        .map((c) => `«${c.title}» ${formatTimeRange(c.starts_at, c.duration_minutes)}`)
        .join(', ')
      toast.warning(`Пересекается с ${list}`)
    })
    .catch(() => {})
}
