'use client'

import { usePayments } from '@/lib/hooks/usePayments'
import type { StudentCourseSummary } from '@/types/api'

function CoursePayments({ course }: { course: StudentCourseSummary }) {
  const { data: payments = [] } = usePayments(course.course_id)
  return (
    <div className="border rounded-xl bg-card p-4">
      <h3 className="text-sm font-semibold mb-3">{course.subject} ({payments.length})</h3>
      {payments.length === 0 ? (
        <p className="text-sm text-muted-foreground">Оплат нет</p>
      ) : (
        <div className="space-y-1">
          {payments.map((p) => (
            <div key={p.id} className="flex items-center justify-between py-1.5 border-b last:border-0 text-sm">
              <span className="text-muted-foreground">{new Date(p.paid_at).toLocaleDateString('ru-RU')}</span>
              <span className="font-medium">{p.amount.toLocaleString()} ₸ · {p.lessons_count} ур.</span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

export function StudentPaymentsTab({ courses }: { courses: StudentCourseSummary[] }) {
  if (courses.length === 0) return <p className="text-sm text-muted-foreground">Нет курсов</p>
  return <div className="space-y-4">{courses.map((c) => <CoursePayments key={c.course_id} course={c} />)}</div>
}
