'use client'

import { Suspense } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'

import { studentApi, LessonsFilter } from '@/lib/api/student'
import { SectionCard, SectionRow } from '@/components/common/SectionCard'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type { CalendarLesson, LessonTask } from '@/types/api'

const dateFmt = new Intl.DateTimeFormat('ru-RU', {
  weekday: 'short',
  day: 'numeric',
  month: 'long',
  hour: '2-digit',
  minute: '2-digit',
})

const STATUS_LABELS: Record<string, string> = {
  scheduled: 'Запланирован',
  completed: 'Проведён',
  cancelled: 'Отменён',
}

function LessonRow({ lesson, isFirst, upcoming }: { lesson: CalendarLesson; isFirst: boolean; upcoming: boolean }) {
  const router = useRouter()
  const queryClient = useQueryClient()

  const tasksKey = ['student-tasks', lesson.id]
  const { data: tasks } = useQuery({
    queryKey: tasksKey,
    queryFn: () => studentApi.tasks(lesson.id),
  })

  const setDone = useMutation({
    mutationFn: ({ id, done }: { id: string; done: boolean }) => studentApi.setTaskDone(id, done),
    onMutate: async ({ id, done }) => {
      await queryClient.cancelQueries({ queryKey: tasksKey })
      const prev = queryClient.getQueryData<LessonTask[]>(tasksKey)
      queryClient.setQueryData<LessonTask[]>(tasksKey, (old) =>
        old?.map((t) => (t.id === id ? { ...t, done } : t)),
      )
      return { prev }
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.prev) queryClient.setQueryData(tasksKey, ctx.prev)
      toast.error('Не удалось обновить задачу')
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: tasksKey }),
  })

  const openBoard = async () => {
    try {
      const { invite_token } = await studentApi.boardToken(lesson.id)
      router.push(`/board/join/${invite_token}`)
    } catch {
      toast.error('Не удалось открыть доску')
    }
  }

  return (
    <SectionRow isFirst={isFirst}>
      <div className="flex items-center justify-between gap-4">
        <div style={{ minWidth: 0 }}>
          <div className="flex items-center gap-2 flex-wrap">
            <span style={{ fontSize: 14, fontWeight: 600 }}>{lesson.subject}</span>
            {lesson.is_group && <Badge variant="secondary">Группа</Badge>}
            {lesson.cycle_position != null && lesson.cycle_size != null && (
              <Badge variant="outline">
                Урок {lesson.cycle_position} из {lesson.cycle_size}
              </Badge>
            )}
            {lesson.paid === true && <Badge variant="secondary">Оплачен</Badge>}
            {lesson.paid === false && <Badge variant="outline">Не оплачен</Badge>}
          </div>
          <div style={{ fontSize: 12.5, color: 'var(--muted-foreground)', marginTop: 2 }}>
            {dateFmt.format(new Date(lesson.scheduled_at))} · {lesson.duration_minutes} мин
            {!upcoming && ` · ${STATUS_LABELS[lesson.status] ?? lesson.status}`}
          </div>
          {tasks && tasks.length > 0 && (
            <div className="flex flex-col gap-1" style={{ marginTop: 8 }}>
              {tasks.map((t) => (
                <label key={t.id} className="flex items-center gap-2" style={{ fontSize: 13 }}>
                  <input
                    type="checkbox"
                    checked={t.done}
                    onChange={(e) => setDone.mutate({ id: t.id, done: e.target.checked })}
                  />
                  <span style={{ textDecoration: t.done ? 'line-through' : 'none' }}>{t.title}</span>
                </label>
              ))}
            </div>
          )}
        </div>
        {upcoming && lesson.status === 'scheduled' && (
          <div className="flex items-center gap-2">
            <Button size="sm" variant="outline" onClick={openBoard}>
              Доска
            </Button>
            <Button size="sm" onClick={() => router.push(`/student/lessons/${lesson.id}/call`)}>
              Войти в урок
            </Button>
          </div>
        )}
      </div>
    </SectionRow>
  )
}

function LessonsInner() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const tab: LessonsFilter = searchParams.get('tab') === 'past' ? 'past' : 'upcoming'

  const { data: lessons, isLoading, error, refetch } = useQuery({
    queryKey: ['student-lessons', tab],
    queryFn: () => studentApi.lessons(tab),
  })

  const cur = lessons?.find((l) => l.cycle_position != null && l.cycle_size != null)
  const cycleSummary =
    cur && cur.cycle_position != null && cur.cycle_size != null
      ? `Оплачено ${cur.cycle_position} из ${cur.cycle_size}, осталось ${cur.cycle_size - cur.cycle_position}`
      : null

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        <Button
          variant={tab === 'upcoming' ? 'default' : 'outline'}
          size="sm"
          onClick={() => router.replace('/student/lessons')}
        >
          Ближайшие
        </Button>
        <Button
          variant={tab === 'past' ? 'default' : 'outline'}
          size="sm"
          onClick={() => router.replace('/student/lessons?tab=past')}
        >
          Прошедшие
        </Button>
      </div>

      {tab === 'upcoming' && cycleSummary && (
        <div style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>{cycleSummary}</div>
      )}

      <SectionCard title={tab === 'upcoming' ? 'Ближайшие уроки' : 'История уроков'}>
        {isLoading && (
          <SectionRow isFirst>
            <span style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>Загрузка...</span>
          </SectionRow>
        )}
        {error != null && !isLoading && (
          <SectionRow isFirst>
            <div className="flex items-center justify-between gap-4">
              <span style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>
                Не удалось загрузить уроки
              </span>
              <Button size="sm" variant="outline" onClick={() => refetch()}>
                Повторить
              </Button>
            </div>
          </SectionRow>
        )}
        {lessons && lessons.length === 0 && (
          <SectionRow isFirst>
            <span style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>
              {tab === 'upcoming' ? 'Ближайших уроков нет' : 'Прошедших уроков нет'}
            </span>
          </SectionRow>
        )}
        {lessons?.map((l, i) => (
          <LessonRow key={l.id} lesson={l} isFirst={i === 0} upcoming={tab === 'upcoming'} />
        ))}
      </SectionCard>
    </div>
  )
}

export default function StudentLessonsPage() {
  return (
    <Suspense>
      <LessonsInner />
    </Suspense>
  )
}
