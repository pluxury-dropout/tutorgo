'use client'

import { HomeworkForm } from '@/components/homework/HomeworkEditPopover'
import type { StudentCourseSummary } from '@/types/api'

export function StudentHomeworkTab({ courses }: { courses: StudentCourseSummary[] }) {
  if (courses.length === 0) return <p className="text-sm text-muted-foreground">Нет курсов</p>
  return (
    <div className="space-y-4">
      {courses.map((c) => (
        <div key={c.course_id} className="border rounded-xl bg-card p-4">
          <h3 className="text-sm font-semibold mb-3">{c.subject}</h3>
          <HomeworkForm courseId={c.course_id} onClose={() => {}} />
        </div>
      ))}
    </div>
  )
}
