'use client'

import { useEffect, useMemo, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { toast } from 'sonner'
import { CalendarDays } from 'lucide-react'

import { studentApi } from '@/lib/api/student'
import { useStudentLessons } from '@/lib/hooks/useStudentLessons'
import { useMinuteTick } from '@/lib/hooks/useMinuteTick'
import { cycleProgress } from '@/lib/cycleProgress'
import { effectiveStatus, STATUS_LABELS } from '@/lib/lessonStatus'
import { SectionCard, SectionRow } from '@/components/common/SectionCard'
import { EmptyState } from '@/components/common/EmptyState'
import { Markdown } from '@/components/common/Markdown'
import { LessonsCalendar } from '@/components/student/LessonsCalendar'
import { isSameLocalDay } from '@/lib/monthGrid'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type { CalendarLesson } from '@/types/api'

const dayTitleFmt = new Intl.DateTimeFormat('ru-RU', {
  weekday: 'long',
  day: 'numeric',
  month: 'long',
})
// Без опции timeZone — Intl рисует в поясе браузера, а scheduled_at приходит
// как TIMESTAMPTZ со смещением. Ученик видит своё локальное время.
const timeFmt = new Intl.DateTimeFormat('ru-RU', { hour: '2-digit', minute: '2-digit' })

function LessonRow({ lesson, isFirst }: { lesson: CalendarLesson; isFirst: boolean }) {
  const status = effectiveStatus(lesson)

  // Вкладку открываем синхронно по клику, иначе popup-блокировщик зарубит
  // window.open после await.
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
            {timeFmt.format(new Date(lesson.scheduled_at))} · {lesson.duration_minutes} мин
            {status !== 'scheduled' && ` · ${STATUS_LABELS[status]}`}
          </div>
        </div>
        {status === 'scheduled' && (
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

/** Полоска «пройдено N из M» по каждому курсу. */
function CycleProgress({ lessons, now }: { lessons: CalendarLesson[]; now: number }) {
  const rows = useMemo(() => cycleProgress(lessons, now), [lessons, now])
  if (rows.length === 0) return null

  return (
    <div style={{ borderTop: '1px solid var(--border)', marginTop: 12, paddingTop: 12 }} className="space-y-2">
      {rows.map((r) => (
        <div key={r.courseId}>
          <div className="flex items-baseline justify-between gap-2" style={{ fontSize: 12 }}>
            <span style={{ fontWeight: 600 }}>{r.subject}</span>
            <span style={{ color: 'var(--muted-foreground)' }}>
              пройдено {r.done} из {r.size} · осталось {Math.max(0, r.size - r.done)}
            </span>
          </div>
          <div
            style={{ height: 5, borderRadius: 999, background: 'var(--muted)', marginTop: 4, overflow: 'hidden' }}
          >
            <div
              style={{
                height: '100%',
                borderRadius: 999,
                background: 'var(--primary)',
                width: `${Math.min(100, (r.done / r.size) * 100)}%`,
              }}
            />
          </div>
        </div>
      ))}
    </div>
  )
}

export default function StudentLessonsPage() {
  const now = useMinuteTick() // статусы уроков в сетке протухают сами по себе
  const { lessons, nextLesson, isLoading, error, refetch } = useStudentLessons()

  const [selected, setSelected] = useState(() => new Date())

  // Один раз после загрузки прыгаем на день ближайшего урока (а если впереди
  // пусто — на последний прошедший), чтобы не открывать пустой месяц.
  const anchored = useRef(false)
  useEffect(() => {
    if (anchored.current || lessons.length === 0) return
    anchored.current = true
    const anchor = nextLesson ?? lessons[lessons.length - 1]
    setSelected(new Date(anchor.scheduled_at))
  }, [lessons, nextLesson])

  const dayLessons = lessons.filter((l) => isSameLocalDay(new Date(l.scheduled_at), selected))

  return (
    // Календарь — вторая колонка на десктопе, но первый блок на мобиле: он
    // управляет тем, что показано ниже, поэтому идёт в разметке раньше.
    <div className="grid gap-4 items-start md:grid-cols-[minmax(0,1fr)_372px]">
      <aside className="md:col-start-2 md:row-start-1">
        {/* Боковые падинги ужаты: каждый пиксель здесь идёт в ширину клетки,
            а в клетке должно читаться время. */}
        <SectionCard bodyPadding="14px 12px">
          {isLoading && (
            <span style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>Загрузка...</span>
          )}
          {error != null && !isLoading && (
            <div className="flex items-center justify-between gap-4">
              <span style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>
                Не удалось загрузить уроки
              </span>
              <Button size="sm" variant="outline" onClick={refetch}>
                Повторить
              </Button>
            </div>
          )}
          {!isLoading && error == null && (
            <>
              <LessonsCalendar lessons={lessons} selected={selected} onSelect={setSelected} now={now} />
              <CycleProgress lessons={lessons} now={now} />
            </>
          )}
        </SectionCard>
      </aside>

      <div className="md:col-start-1 md:row-start-1 space-y-4">
        <HomeworkSection />

        {!isLoading && error == null && (
          <SectionCard title={dayTitleFmt.format(selected)}>
            {dayLessons.length === 0 ? (
              lessons.length === 0 ? (
                <EmptyState
                  size="sm"
                  icon={CalendarDays}
                  title="Уроков пока нет"
                  description="Как только преподаватель поставит урок, он появится в календаре — с датой, предметом и кнопкой входа в звонок"
                />
              ) : (
                <SectionRow isFirst>
                  <span style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>
                    В этот день уроков нет — выбери день с меткой в календаре
                  </span>
                </SectionRow>
              )
            ) : (
              dayLessons.map((l, i) => <LessonRow key={l.id} lesson={l} isFirst={i === 0} />)
            )}
          </SectionCard>
        )}
      </div>
    </div>
  )
}
