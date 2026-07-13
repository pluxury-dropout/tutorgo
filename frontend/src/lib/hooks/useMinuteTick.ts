import { useEffect, useState } from 'react'

// Форсит ре-рендер раз в минуту, чтобы effectiveStatus пересчитал бейджи
// без действий пользователя (урок «проведён» в момент окончания).
export function useMinuteTick() {
  const [, force] = useState(0)
  useEffect(() => {
    const id = setInterval(() => force(n => n + 1), 60_000)
    return () => clearInterval(id)
  }, [])
}
