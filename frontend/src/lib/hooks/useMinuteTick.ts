import { useEffect, useState } from 'react'

// Отдаёт «сейчас» и обновляет его раз в минуту, чтобы производные от времени
// вещи (effectiveStatus, окно входа в урок, подсветка сегодняшнего дня)
// пересчитывались без действий пользователя. Возвращаемое значение можно
// игнорировать — тогда хук работает просто как форс ре-рендера.
export function useMinuteTick(): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 60_000)
    return () => clearInterval(id)
  }, [])
  return now
}
