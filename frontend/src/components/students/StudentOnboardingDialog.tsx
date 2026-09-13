'use client'

import { useState } from 'react'
import { toast } from 'sonner'

import { useOnboardStudent } from '@/lib/hooks/useStudents'
import { useCourses } from '@/lib/hooks/useCourses'
import { WEEK_DAYS, isoWeekday } from '@/lib/recurrence'
import type { ApiError, Course, OnboardingStudentInput } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { TimePicker } from '@/components/ui/time-picker'
import { SubjectCombobox } from '@/components/courses/SubjectCombobox'

interface Props {
  open:    boolean
  onClose: () => void
}

const pad = (n: number) => String(n).padStart(2, '0')

/** Дефолт для полей оплаты — самый частый ПАКЕТ тьютора: пара
 *  (price_per_cycle, lessons_per_cycle), а не частая цена и частое число
 *  уроков по отдельности — иначе сумма одного пакета склеится с количеством
 *  другого. Ничья по частоте — в пользу пакета с более поздним started_at,
 *  дальше сортировка детерминирована (по цене, потом по числу уроков).
 *  Правило зеркалит SQL в repository/course.go:GetOrCreateIndividual —
 *  менять оба места нужно вместе. */
function mostCommonPackage(courses: Course[]): { price: number | ''; lessons: number } {
  if (courses.length === 0) return { price: '', lessons: 1 }

  const groups = new Map<string, { price: number; lessons: number; count: number; latest: number }>()
  for (const c of courses) {
    const key       = `${c.price_per_cycle}:${c.lessons_per_cycle}`
    const startedAt = new Date(c.started_at).getTime()
    const group     = groups.get(key)
    if (group) {
      group.count++
      if (startedAt > group.latest) group.latest = startedAt
    } else {
      groups.set(key, { price: c.price_per_cycle, lessons: c.lessons_per_cycle, count: 1, latest: startedAt })
    }
  }

  const [best] = [...groups.values()].sort((a, b) =>
    b.count - a.count || b.latest - a.latest || a.price - b.price || a.lessons - b.lessons,
  )
  return { price: best.price, lessons: best.lessons }
}

/** Быстрый онбординг (спека 8.2, п. 8.2): один сабмит заводит ученика, а при
 *  заполненном предмете — ещё и курс с серией уроков на дефолтных днях/времени.
 *  Условный рендер формы вместо reset-эффекта на `open`: компонент монтируется
 *  заново при каждом открытии, поэтому дефолты (предмет — по первому курсу
 *  тьютора, оплата — по самому частому пакету, см. mostCommonPackage) читаются
 *  сразу в useState без гонки с уже открытой формой при фоновом рефетче
 *  курсов. */
export function StudentOnboardingDialog({ open, onClose }: Props) {
  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Новый ученик</DialogTitle>
        </DialogHeader>
        {open && <OnboardingForm onClose={onClose} />}
      </DialogContent>
    </Dialog>
  )
}

function OnboardingForm({ onClose }: { onClose: () => void }) {
  // useCourses отдаёт только активные курсы (первые 100 — см. coursesApi.list),
  // агрегирующей ручки на бэке для статистики по пакетам нет; для формы
  // онбординга это overkill — тьютор всё видит и может поправить перед сабмитом.
  const { data: courses = [] } = useCourses()
  const onboard = useOnboardStudent()
  const defaultPkg = mostCommonPackage(courses)

  const [firstName, setFirstName] = useState('')
  const [phone, setPhone]         = useState('')
  const [subject, setSubject]     = useState(courses[0]?.subject ?? '')
  const [days, setDays]           = useState<number[]>([isoWeekday(new Date())])
  const [hour, setHour]           = useState('17')
  const [minute, setMinute]       = useState('0')
  const [duration, setDuration]   = useState(60)
  // Пара «сумма за пакет» + «уроков в пакете» — те же единицы, что хранит
  // курс (price_per_cycle/lessons_per_cycle), без домножений и делений.
  const [price, setPrice]         = useState<number | ''>(defaultPkg.price)
  const [lessons, setLessons]     = useState<number>(defaultPkg.lessons)
  const [error, setError]         = useState('')

  function toggleDay(iso: number) {
    setDays((prev) => (prev.includes(iso) ? prev.filter((d) => d !== iso) : [...prev, iso]))
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const name = firstName.trim()
    if (name.length < 2) { setError('Минимум 2 символа'); return }
    if (phone.trim() && phone.trim().length < 10) { setError('Телефон — минимум 10 символов'); return }
    // Очищенное поле даёт Number('') = 0, бэкенд пропускает 0 через omitempty,
    // а сервис онбординга подставляет 1 — и пакет «80 000 за 12» молча стал бы
    // «80 000 за 1 урок». Для денег лучше остановить форму, чем угадывать.
    if (subject.trim() && price !== '' && (!Number.isInteger(lessons) || lessons < 1)) {
      setError('Уроков в пакете — целое число от 1')
      return
    }
    setError('')

    // Без предмета — только ученик; предмет задан, но дни не выбраны —
    // предмет без расписания. Ветвление зеркалит service/onboarding.go.
    const data: OnboardingStudentInput = { first_name: name }
    if (phone.trim()) data.phone = phone.trim()

    if (subject.trim()) {
      data.subject = subject.trim()
      // Канон price_per_cycle/lessons_per_cycle как в CourseForm — сумма и
      // число уроков уходят как есть, без домножений на стороне клиента.
      if (price !== '') {
        data.price_per_cycle   = price
        data.lessons_per_cycle = lessons
      }
      if (days.length > 0) {
        const now     = new Date()
        const dateStr = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`
        const timeStr = `${pad(Number(hour))}:${pad(Number(minute))}`
        data.schedule = {
          byweekday:        days,
          time_local:       timeStr,
          tz:               Intl.DateTimeFormat().resolvedOptions().timeZone,
          duration_minutes: duration,
          // Инстант, а не голая дата: gin разбирает time.Time только из полного
          // RFC3339 со смещением. Календарное число сервер вычитывает уже в
          // зоне тьютора, так что UTC здесь ничего не сдвигает.
          starts_on: new Date(`${dateStr}T${timeStr}`).toISOString(),
        }
      }
    }

    try {
      const res = await onboard.mutateAsync(data)
      toast.success(
        res.lessons_created > 0
          ? `Ученик добавлен, создано уроков: ${res.lessons_created}`
          : 'Ученик добавлен',
      )
      onClose()
    } catch (err) {
      const apiErr = err as ApiError
      toast.error(apiErr.message ?? 'Ошибка сохранения')
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4 pt-2">
      <div className="space-y-1.5">
        <Label htmlFor="onb_first_name">Имя</Label>
        <Input
          id="onb_first_name"
          autoFocus
          value={firstName}
          onChange={(e) => setFirstName(e.target.value)}
        />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="onb_phone">
          Телефон <span className="text-muted-foreground font-normal">(необязательно)</span>
        </Label>
        <Input
          id="onb_phone"
          type="tel"
          placeholder="+77001234567"
          value={phone}
          onChange={(e) => setPhone(e.target.value)}
        />
      </div>
      {error && <p className="text-xs text-destructive">{error}</p>}

      <Separator />

      <div className="space-y-1.5">
        <Label>Предмет <span className="text-muted-foreground font-normal">(необязательно)</span></Label>
        <SubjectCombobox value={subject} onChange={setSubject} />
      </div>

      {subject.trim() && (
        <>
          <div className="space-y-1.5">
            <Label>Когда</Label>
            <div className="flex gap-1">
              {WEEK_DAYS.map(({ label, iso }) => (
                <button
                  key={iso}
                  type="button"
                  onClick={() => toggleDay(iso)}
                  className={`h-8 w-9 rounded text-xs font-medium transition-colors ${
                    days.includes(iso)
                      ? 'bg-primary text-primary-foreground'
                      : 'border border-input hover:bg-muted'
                  }`}
                >
                  {label}
                </button>
              ))}
            </div>
            <div className="flex items-center gap-2 pt-1">
              <TimePicker hour={hour} minute={minute} onHourChange={setHour} onMinuteChange={setMinute} />
              <Input
                type="number"
                min={5}
                step={5}
                className="w-16"
                value={duration}
                onChange={(e) => setDuration(Number(e.target.value))}
              />
              <span className="text-xs text-muted-foreground">мин</span>
            </div>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="onb_price">Оплата</Label>
            <div className="flex items-center gap-1.5">
              <Input
                id="onb_price"
                type="number"
                min={0}
                step="any"
                className="w-28"
                value={price}
                onChange={(e) => setPrice(e.target.value === '' ? '' : Number(e.target.value))}
              />
              <span className="text-xs text-muted-foreground">₸ за</span>
              <Input
                id="onb_lessons"
                type="number"
                min={1}
                step={1}
                className="w-16"
                value={lessons}
                onChange={(e) => setLessons(Number(e.target.value))}
              />
              <span className="text-xs text-muted-foreground">уроков</span>
            </div>
            {lessons > 1 && typeof price === 'number' && price > 0 && (
              <p className="text-xs text-muted-foreground">
                ≈ {Math.round(price / lessons).toLocaleString()} ₸ за урок
              </p>
            )}
          </div>
        </>
      )}

      <Separator />

      <div className="flex justify-end gap-2">
        <Button type="button" variant="outline" onClick={onClose}>Отмена</Button>
        <Button type="submit" disabled={onboard.isPending}>
          {onboard.isPending ? 'Добавление...' : 'Добавить'}
        </Button>
      </div>
    </form>
  )
}
