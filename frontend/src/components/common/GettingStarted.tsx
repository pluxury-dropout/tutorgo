import Link from 'next/link'
import { Check } from 'lucide-react'
import { SectionCard, SectionRow } from '@/components/common/SectionCard'
import { buttonVariants } from '@/components/ui/button'

interface GettingStartedProps {
  hasStudents: boolean
  hasCourses: boolean
  hasLessons: boolean
  hasPayments: boolean
}

/**
 * Чеклист первых шагов для нового преподавателя. Состояние шагов — производное
 * от данных, которые главная и так грузит: отдельного флага «онбординг пройден»
 * нет и не нужно. Когда все четыре шага выполнены, блок пропадает навсегда.
 */
export function GettingStarted({ hasStudents, hasCourses, hasLessons, hasPayments }: GettingStartedProps) {
  const steps = [
    { done: hasStudents, label: 'Добавить ученика',           hint: 'Карточка с контактами — к ней привяжутся курсы и оплаты', href: '/courses?tab=students', cta: 'Добавить' },
    { done: hasCourses,  label: 'Создать курс',               hint: 'Предмет и цена за урок: из курса растут расписание и деньги', href: '/courses',              cta: 'Создать'  },
    { done: hasLessons,  label: 'Поставить урок в расписание', hint: 'Кликни по свободному слоту в календаре',                   href: '/calendar',             cta: 'В календарь' },
    { done: hasPayments, label: 'Отметить оплату',            hint: 'Приложение само посчитает, на сколько уроков хватит баланса', href: '/payments',             cta: 'К оплатам' },
  ]

  if (steps.every(s => s.done)) return null

  const nextIdx = steps.findIndex(s => !s.done)

  return (
    <SectionCard title="Первые шаги">
      {steps.map((step, i) => (
        <SectionRow key={step.label} isFirst={i === 0} style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <span style={{
            width: 18, height: 18, borderRadius: '50%', flexShrink: 0,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            background: step.done ? 'var(--success)' : 'transparent',
            border: step.done ? 'none' : '1.5px solid var(--border)',
          }}>
            {step.done && <Check size={11} strokeWidth={3} color="var(--card)" />}
          </span>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{
              fontSize: 13, fontWeight: 600,
              color: step.done ? 'var(--muted-foreground)' : 'var(--foreground)',
              textDecoration: step.done ? 'line-through' : 'none',
            }}>
              {step.label}
            </div>
            {!step.done && (
              <div style={{ fontSize: 12, color: 'var(--muted-foreground)' }}>{step.hint}</div>
            )}
          </div>
          {i === nextIdx && (
            <Link href={step.href} className={buttonVariants({ size: 'sm' })} style={{ flexShrink: 0 }}>
              {step.cta}
            </Link>
          )}
        </SectionRow>
      ))}
    </SectionCard>
  )
}
