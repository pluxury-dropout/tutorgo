'use client'

import { Suspense, useEffect, useRef, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { useQuery } from '@tanstack/react-query'
import { toast } from 'sonner'
import { CalendarDays } from 'lucide-react'

import { studentApi, LessonsFilter } from '@/lib/api/student'
import { SectionCard, SectionRow } from '@/components/common/SectionCard'
import { EmptyState } from '@/components/common/EmptyState'
import { Markdown } from '@/components/common/Markdown'
import { PeriodPicker } from '@/components/lessons/PeriodPicker'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type { CalendarLesson } from '@/types/api'

// Неделя (Пн–Пн+7), содержащая переданную дату
function weekRangeOf(d: Date): { from: Date; to: Date } {
  const day = d.getDay()
  const diff = day === 0 ? -6 : 1 - day
  const from = new Date(d)
  from.setDate(d.getDate() + diff)
  from.setHours(0, 0, 0, 0)
  const to = new Date(from)
  to.setDate(from.getDate() + 7)
  return { from, to }
}

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
  // Вкладку открываем синхронно по клику, иначе popup-блокировщик зарубит window.open после await.
  const openBoard = () => {
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
        </div>
        {upcoming && lesson.status === 'scheduled' && (
          <div className="flex items-center gap-2">
            <Button size="sm" variant="outline" onClick={openBoard}>
              Доска
            </Button>
            <Button size="sm" onClick={() => window.open(`/student/lessons/${lesson.id}/call`, '_blank')}>
              Войти в урок
            </Button>
          </div>
        )}
      </div>
    </SectionRow>
  )
}

function HomeworkSection() {
  const { data: homework } = useQuery({
    queryKey: ['student-homework'],
    queryFn: () => studentApi.homework(),
  })

  if (!homework || homework.length === 0) return null

  return (
    <SectionCard title="Домашнее задание">
      {homework.map((h, i) => (
        <SectionRow key={h.course_id} isFirst={i === 0}>
          <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 6 }}>{h.subject}</div>
          <Markdown>{h.homework}</Markdown>
        </SectionRow>
      ))}
    </SectionCard>
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

  // Недельный срез. Дефолт — неделя ближайшего/последнего урока (список уже отсортирован),
  // чтобы не открывать пустой экран. Сбрасывается один раз на вкладку.
  const [period, setPeriod] = useState(() => weekRangeOf(new Date()))
  const initedTab = useRef<string | null>(null)
  useEffect(() => {
    if (!lessons || initedTab.current === tab) return
    initedTab.current = tab
    setPeriod(weekRangeOf(lessons[0] ? new Date(lessons[0].scheduled_at) : new Date()))
  }, [lessons, tab])

  const visible = (lessons ?? []).filter((l) => {
    const t = new Date(l.scheduled_at)
    return t >= period.from && t < period.to
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

      {tab === 'upcoming' && <HomeworkSection />}

      {tab === 'upcoming' && cycleSummary && (
        <div style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>{cycleSummary}</div>
      )}

      <SectionCard
        title={tab === 'upcoming' ? 'Ближайшие уроки' : 'История уроков'}
        action={<PeriodPicker from={period.from} to={period.to} onChange={(f, t) => setPeriod({ from: f, to: t })} />}
        style={{ overflow: 'visible' }}
      >
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
        {lessons && !isLoading && error == null && visible.length === 0 && (
          lessons.length === 0 ? (
            <EmptyState
              size="sm"
              icon={CalendarDays}
              title={tab === 'upcoming' ? 'Ближайших уроков нет' : 'Прошедших уроков нет'}
              description={tab === 'upcoming'
                ? 'Как только преподаватель поставит урок, он появится здесь — с датой, предметом и кнопкой входа в звонок'
                : 'Здесь будет история проведённых уроков вместе с домашними заданиями к ним'}
            />
          ) : (
            <EmptyState
              size="sm"
              icon={CalendarDays}
              title="На выбранной неделе уроков нет"
              description="Уроки есть в другие недели — пролистай период выше"
            />
          )
        )}
        {visible.map((l, i) => (
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
