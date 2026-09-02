'use client'

import * as React from 'react'
import { useState } from 'react'
import { toast } from 'sonner'
import { AlertTriangle, Repeat } from 'lucide-react'

import { useCreateTask } from '@/lib/hooks/useTasks'
import { useCreateEvent, useConflicts } from '@/lib/hooks/useEvents'
import { useCreateSlotLesson } from '@/lib/hooks/useLessons'
import { useStudentCourses } from '@/lib/hooks/useCourses'
import { toRecurrenceInput, isoWeekday, WEEK_DAYS } from '@/lib/recurrence'
import { EVENT_KINDS, KIND_LABELS, formatTimeRange } from '@/lib/eventKind'
import type { EventKind, Student } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTitle } from '@/components/ui/popover'
import { Input } from '@/components/ui/input'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { StudentCombobox } from '@/components/students/StudentCombobox'
import { SubjectCombobox } from '@/components/courses/SubjectCombobox'

interface Props {
  start:   Date | null
  end:     Date | null
  /** Прямоугольник выделенного слота: сама подсветка снимается сразу после
   *  выделения, привязываться не к чему — поэтому храним rect, а не узел. */
  anchor:  DOMRect | null
  onClose: () => void
}

function formatSlot(start: Date, end: Date): string {
  const day = start.toLocaleDateString('ru-RU', { weekday: 'short', day: 'numeric', month: 'short' })
  const t1  = start.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
  const t2  = end.toLocaleTimeString('ru-RU',   { hour: '2-digit', minute: '2-digit' })
  return `${day} · ${t1} – ${t2}`
}

function slotMinutes(start: Date, end: Date): number {
  return Math.max(15, Math.round((end.getTime() - start.getTime()) / 60_000))
}

export function SlotCreatePopover({ start, end, anchor, onClose }: Props) {
  return (
    <Popover open={!!start} onOpenChange={(open) => !open && onClose()}>
      <PopoverContent anchor={anchor}>
        <PopoverTitle className="pr-8">Новая запись</PopoverTitle>
        {/* key: новый слот пересоздаёт форму — поля чистые без эффекта.
            Фокус в поле ставит сам поповер: это первый tabbable-элемент. */}
        {start && end && (
          <SlotForm key={start.toISOString()} start={start} end={end} onClose={onClose} />
        )}
      </PopoverContent>
    </Popover>
  )
}

function SlotForm({ start, end, onClose }: { start: Date; end: Date; onClose: () => void }) {
  // Тип не запоминается между открытиями: иначе репетитор, один раз создавший
  // задачу, потом молча создаёт задачи вместо уроков. Дефолт — урок: это то,
  // ради чего в календарь вообще тыкают.
  const [tab, setTab] = useState('lesson')

  // Занятость запрашиваем сразу при открытии, до заполнения полей.
  const { data: conflicts = [] } = useConflicts({
    starts_at:        start.toISOString(),
    duration_minutes: slotMinutes(start, end),
  })

  return (
    <>
      <p className="text-xs text-muted-foreground -mt-2">{formatSlot(start, end)}</p>

      {conflicts.length > 0 && (
        <p className="flex items-start gap-1.5 rounded-md bg-[var(--cal-missed-bg)] px-2 py-1.5 text-xs text-[var(--cal-missed-text)]">
          <AlertTriangle className="mt-px size-3.5 shrink-0" />
          <span>
            Занято: {conflicts.map((c) => `${c.title} ${formatTimeRange(c.starts_at, c.duration_minutes)}`).join(', ')}
          </span>
        </p>
      )}

      <Tabs value={tab} onValueChange={(v) => setTab(v as string)}>
        <TabsList className="w-full">
          <TabsTrigger value="lesson">Урок</TabsTrigger>
          <TabsTrigger value="event">Событие</TabsTrigger>
          <TabsTrigger value="task">Задача</TabsTrigger>
        </TabsList>

        <TabsContent value="lesson" className="pt-3">
          <LessonFields start={start} end={end} onClose={onClose} />
        </TabsContent>
        <TabsContent value="event" className="pt-3">
          <EventFields start={start} end={end} onClose={onClose} />
        </TabsContent>
        <TabsContent value="task" className="pt-3">
          <TaskFields start={start} end={end} onClose={onClose} />
        </TabsContent>
      </Tabs>
    </>
  )
}

/** Состояние повтора, общее для урока и события: правило уезжает на сервер,
 *  раскатывать даты на клиенте больше не нужно. */
function useRepeat(start: Date) {
  const [mode, setMode] = useState<'none' | 'weekly_same' | 'weekly_custom'>('none')
  const [days, setDays] = useState<number[]>([isoWeekday(start)])

  // Пустой список дней — не молчаливая серия из одного элемента, а
  // заблокированная кнопка: молчаливый один урок выглядит как потеря данных.
  const missing = mode === 'weekly_custom' && days.length === 0

  return {
    mode, setMode, days, setDays, missing,
    recurrence: mode === 'none' || missing
      ? undefined
      : toRecurrenceInput({ type: mode, days }),
  }
}

type Repeat = ReturnType<typeof useRepeat>

function RepeatPicker({ repeat }: { repeat: Repeat }) {
  const { mode, setMode, days, setDays, missing } = repeat

  function toggleDay(iso: number) {
    setDays((prev) => (prev.includes(iso) ? prev.filter((d) => d !== iso) : [...prev, iso]))
  }

  if (mode === 'none') {
    return (
      <button
        type="button"
        onClick={() => setMode('weekly_same')}
        className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground"
      >
        <Repeat className="size-3.5" />
        Повторять
      </button>
    )
  }

  return (
    <div className="space-y-2 rounded-lg border border-input p-2">
      <div className="flex gap-1">
        <RepeatMode active={mode === 'weekly_same'} onClick={() => setMode('weekly_same')}>
          Еженедельно
        </RepeatMode>
        <RepeatMode active={mode === 'weekly_custom'} onClick={() => setMode('weekly_custom')}>
          По дням
        </RepeatMode>
      </div>

      {mode === 'weekly_custom' && (
        <div className="flex gap-1">
          {WEEK_DAYS.map(({ label, iso }) => (
            <button
              key={iso}
              type="button"
              onClick={() => toggleDay(iso)}
              className={`h-7 flex-1 rounded text-xs font-medium transition-colors ${
                days.includes(iso)
                  ? 'bg-primary text-primary-foreground'
                  : 'border border-input hover:bg-muted'
              }`}
            >
              {label}
            </button>
          ))}
        </div>
      )}

      {missing && <p className="text-xs text-destructive">Выберите хотя бы один день недели</p>}

      <button
        type="button"
        onClick={() => setMode('none')}
        className="text-xs text-muted-foreground hover:text-foreground"
      >
        Не повторять
      </button>
    </div>
  )
}

function RepeatMode({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`h-7 flex-1 rounded text-xs font-medium transition-colors ${
        active ? 'bg-primary text-primary-foreground' : 'border border-input hover:bg-muted'
      }`}
    >
      {children}
    </button>
  )
}

function LessonFields({ start, end, onClose }: { start: Date; end: Date; onClose: () => void }) {
  const [student, setStudent] = useState<Student | null>(null)
  // null — пользователь предмет не трогал, показываем подсказку по ученику.
  // Состояние вместо эффекта: подстановка при загрузке курсов иначе была бы
  // setState внутри useEffect, то есть лишний каскад рендеров.
  const [typedSubject, setTypedSubject] = useState<string | null>(null)
  const repeat = useRepeat(start)
  const createLesson = useCreateSlotLesson()

  // Предмет подставляем из последнего курса ученика: у большинства он один,
  // и печатать его заново на каждый урок незачем.
  const { data: studentCourses = [] } = useStudentCourses(student?.id ?? '')
  const subject = typedSubject ?? studentCourses[0]?.subject ?? ''

  function handleSave() {
    if (!student || !subject.trim() || repeat.missing) return
    createLesson.mutate(
      {
        student_id:       student.id,
        subject:          subject.trim(),
        scheduled_at:     start.toISOString(),
        duration_minutes: slotMinutes(start, end),
        recurrence:       repeat.recurrence,
      },
      { onError: () => toast.error('Не удалось поставить урок') },
    )
    onClose()
  }

  return (
    <div className="space-y-3">
      <StudentCombobox
        value={student}
        onChange={(s) => { setStudent(s); setTypedSubject(null) }}
        autoFocus
      />
      {student && <SubjectCombobox value={subject} onChange={setTypedSubject} />}

      <RepeatPicker repeat={repeat} />

      <div className="flex items-center justify-end gap-2">
        <Button variant="ghost" size="sm" onClick={onClose}>Отмена</Button>
        <Button
          size="sm"
          onClick={handleSave}
          disabled={!student || !subject.trim() || repeat.missing}
        >
          {repeat.mode === 'none' ? 'Создать' : 'Создать серию'}
        </Button>
      </div>
    </div>
  )
}

function EventFields({ start, end, onClose }: { start: Date; end: Date; onClose: () => void }) {
  const [title, setTitle] = useState('')
  const [kind, setKind]   = useState<EventKind>('personal')
  const repeat = useRepeat(start)
  const createEvent = useCreateEvent()

  function handleSave() {
    if (!title.trim() || repeat.missing) return
    createEvent.mutate(
      {
        title:            title.trim(),
        kind,
        starts_at:        start.toISOString(),
        duration_minutes: slotMinutes(start, end),
        recurrence:       repeat.recurrence,
      },
      { onError: () => toast.error('Не удалось создать событие') },
    )
    onClose()
  }

  return (
    <div className="space-y-3">
      <Input
        placeholder="Название события"
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        onKeyDown={(e) => e.key === 'Enter' && handleSave()}
      />
      <div className="flex gap-1">
        {EVENT_KINDS.map((k) => (
          <button
            key={k}
            type="button"
            onClick={() => setKind(k)}
            className={`h-7 flex-1 rounded text-xs font-medium transition-colors ${
              kind === k ? 'bg-primary text-primary-foreground' : 'border border-input hover:bg-muted'
            }`}
          >
            {KIND_LABELS[k]}
          </button>
        ))}
      </div>

      <RepeatPicker repeat={repeat} />

      <div className="flex justify-end gap-2">
        <Button variant="ghost" size="sm" onClick={onClose}>Отмена</Button>
        <Button size="sm" onClick={handleSave} disabled={!title.trim() || repeat.missing}>
          {repeat.mode === 'none' ? 'Создать' : 'Создать серию'}
        </Button>
      </div>
    </div>
  )
}

function TaskFields({ start, end, onClose }: { start: Date; end: Date; onClose: () => void }) {
  const [title, setTitle] = useState('')
  const createTask        = useCreateTask()

  function handleSave() {
    if (!title.trim()) return
    createTask.mutate(
      { title: title.trim(), scheduled_at: start.toISOString(), duration_minutes: slotMinutes(start, end) },
      { onError: () => toast.error('Не удалось создать задачу') },
    )
    onClose()
  }

  return (
    <div className="space-y-3">
      <Input
        placeholder="Название задачи"
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        onKeyDown={(e) => e.key === 'Enter' && handleSave()}
      />
      <div className="flex justify-end gap-2">
        <Button variant="ghost" size="sm" onClick={onClose}>Отмена</Button>
        <Button size="sm" onClick={handleSave} disabled={!title.trim()}>Создать</Button>
      </div>
    </div>
  )
}
