import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'

import { studentApi } from '@/lib/api/student'
import type { CalendarLesson } from '@/types/api'

// Ручка /student/lessons умеет только filter=upcoming|past — это два разных
// WHERE по now() в repository/student.go. Календарю нужны оба конца, поэтому
// дёргаем обе выборки и склеиваем на клиенте: rank (а значит и позиция в цикле)
// считается в SQL по всем неотменённым урокам курса, без date-фильтра, так что
// нумерация после склейки остаётся согласованной.
const upcomingQuery = {
  queryKey: ['student-lessons', 'upcoming'],
  queryFn: () => studentApi.lessons('upcoming'),
}

const pastQuery = {
  queryKey: ['student-lessons', 'past'],
  queryFn: () => studentApi.lessons('past'),
}

/**
 * Ближайший непроведённый урок. Отдельный хук ради шапки кабинета: ей незачем
 * тянуть историю, а ключ тот же, что у страницы уроков — запрос переиспользуется.
 */
export function useNextLesson(): CalendarLesson | undefined {
  const { data } = useQuery(upcomingQuery)
  // Бэкенд отдаёт upcoming по возрастанию времени, но отменённые уроки из
  // выборки не убирает — их нужно пропустить.
  return data?.find((l) => l.status === 'scheduled')
}

/** Вся лента уроков ученика (прошлое + будущее) по возрастанию времени. */
export function useStudentLessons() {
  const upcoming = useQuery(upcomingQuery)
  const past = useQuery(pastQuery)

  const lessons = useMemo(
    () =>
      [...(past.data ?? []), ...(upcoming.data ?? [])].sort(
        // Сравниваем моменты времени, а не строки: смещение в RFC3339 может
        // отличаться, и лексикографический порядок тогда врёт.
        (a, b) => new Date(a.scheduled_at).getTime() - new Date(b.scheduled_at).getTime(),
      ),
    [past.data, upcoming.data],
  )

  return {
    lessons,
    nextLesson: upcoming.data?.find((l) => l.status === 'scheduled'),
    isLoading: upcoming.isLoading || past.isLoading,
    error: upcoming.error ?? past.error,
    refetch: () => {
      void upcoming.refetch()
      void past.refetch()
    },
  }
}
