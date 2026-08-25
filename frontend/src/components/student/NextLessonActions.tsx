'use client'

import { useRouter } from 'next/navigation'
import { PenLine, Video } from 'lucide-react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { studentApi } from '@/lib/api/student'
import { useMinuteTick } from '@/lib/hooks/useMinuteTick'
import { useNextLesson } from '@/lib/hooks/useStudentLessons'
import { isSameLocalDay } from '@/lib/monthGrid'

// За сколько до начала пускаем в комнату.
const JOIN_WINDOW_MS = 15 * 60_000

const timeFmt = new Intl.DateTimeFormat('ru-RU', { hour: '2-digit', minute: '2-digit' })
const dayFmt = new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'short' })

/** Кнопки шапки кабинета: доска и вход в звонок ближайшего урока. */
export function NextLessonActions() {
  const now = useMinuteTick() // окно входа открывается само, без перезагрузки
  const router = useRouter()
  const lesson = useNextLesson()

  // Вкладку открываем синхронно по клику, иначе popup-блокировщик зарубит
  // window.open после await.
  const openBoard = () => {
    if (!lesson) {
      // Уроков впереди нет — ведём к списку курсов, доски там же.
      router.push('/student/board')
      return
    }
    const w = window.open('', '_blank')
    studentApi
      .boardToken(lesson.id)
      .then(({ invite_token }) => {
        if (w) w.location.href = `/board/join/${invite_token}`
      })
      .catch(() => {
        w?.close()
        toast.error('Не удалось открыть доску')
      })
  }

  const start = lesson ? new Date(lesson.scheduled_at) : null
  const joinable =
    start != null &&
    now >= start.getTime() - JOIN_WINDOW_MS &&
    now < start.getTime() + (lesson?.duration_minutes ?? 0) * 60_000

  const startsToday = start != null && isSameLocalDay(new Date(now), start)
  const joinLabel = joinable
    ? 'Войти в урок'
    : start == null
      ? ''
      : startsToday
        ? `В ${timeFmt.format(start)}`
        : `${dayFmt.format(start)}, ${timeFmt.format(start)}`

  return (
    <>
      <Button size="sm" variant="outline" onClick={openBoard} title="Доска">
        <PenLine className="h-3.5 w-3.5" />
        <span className="hidden sm:inline">Доска</span>
      </Button>
      {lesson && start && (
        <Button
          size="sm"
          disabled={!joinable}
          title={joinable ? 'Войти в урок' : `Урок начнётся ${dayFmt.format(start)} в ${timeFmt.format(start)}`}
          onClick={() => window.open(`/student/lessons/${lesson.id}/call`, '_blank')}
        >
          <Video className="h-3.5 w-3.5" />
          {joinLabel}
        </Button>
      )}
    </>
  )
}
