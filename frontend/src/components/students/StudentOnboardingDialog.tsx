'use client'

import { useState } from 'react'
import { toast } from 'sonner'

import { useOnboardStudent } from '@/lib/hooks/useStudents'
import { useCourses } from '@/lib/hooks/useCourses'
import { WEEK_DAYS, isoWeekday } from '@/lib/recurrence'
import type { ApiError, OnboardingStudentInput } from '@/types/api'

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

/** Быстрый онбординг (спека 8.2, п. 8.2): один сабмит заводит ученика, а при
 *  заполненном предмете — ещё и курс с серией уроков на дефолтных днях/времени.
 *  Условный рендер формы вместо reset-эффекта на `open`: компонент монтируется
 *  заново при каждом открытии, поэтому дефолты (предмет и цена — по первому
 *  курсу тьютора) читаются сразу в useState без гонки с уже открытой формой
 *  при фоновом рефетче курсов. */
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
  // "Самый частый" курс в первом приближении — просто первый из списка:
  // агрегирующей ручки на бэке для этого нет, а точная статистика для формы
  // онбординга overkill — тьютор всё видит и может поправить перед сабмитом.
  const { data: courses = [] } = useCourses()
  const onboard = useOnboardStudent()

  const [firstName, setFirstName] = useState('')
  const [phone, setPhone]         = useState('')
  const [subject, setSubject]     = useState(courses[0]?.subject ?? '')
  const [days, setDays]           = useState<number[]>([isoWeekday(new Date())])
  const [hour, setHour]           = useState('17')
  const [minute, setMinute]       = useState('0')
  const [duration, setDuration]   = useState(60)
  // Поле спрашивает цену за урок, а курс хранит цену за цикл — делим, иначе у
  // тьютора с циклом в 8 уроков в поле подставится восьмикратная цена.
  // Обратное умножение при сабмите не нужно: цикл этого поля всегда равен
  // одному уроку (lessons_per_cycle: 1 ниже), а не lessons_per_cycle курса-донора.
  const [price, setPrice]         = useState<number | ''>(
    courses[0] ? Math.round(courses[0].price_per_cycle / (courses[0].lessons_per_cycle || 1)) : '',
  )
  const [error, setError]         = useState('')

  function toggleDay(iso: number) {
    setDays((prev) => (prev.includes(iso) ? prev.filter((d) => d !== iso) : [...prev, iso]))
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const name = firstName.trim()
    if (name.length < 2) { setError('Минимум 2 символа'); return }
    if (phone.trim() && phone.trim().length < 10) { setError('Телефон — минимум 10 символов'); return }
    setError('')

    // Без предмета — только ученик; предмет задан, но дни не выбраны —
    // предмет без расписания. Ветвление зеркалит service/onboarding.go.
    const data: OnboardingStudentInput = { first_name: name }
    if (phone.trim()) data.phone = phone.trim()

    if (subject.trim()) {
      data.subject = subject.trim()
      // Поле выше в единицах «за урок» — цикл равен одному уроку явно,
      // а не молчаливым дефолтом service/onboarding.go.
      if (price !== '') {
        data.price_per_cycle   = price
        data.lessons_per_cycle = 1
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
              <span className="text-xs text-muted-foreground">₸ за урок</span>
            </div>
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
