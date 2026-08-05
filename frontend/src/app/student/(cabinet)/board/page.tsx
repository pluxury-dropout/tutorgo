'use client'

import { useQuery } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Presentation } from 'lucide-react'

import { studentApi } from '@/lib/api/student'
import { SectionCard, SectionRow } from '@/components/common/SectionCard'
import { EmptyState } from '@/components/common/EmptyState'
import { Button } from '@/components/ui/button'
import type { StudentCourse } from '@/types/api'

function CourseRow({ course, isFirst }: { course: StudentCourse; isFirst: boolean }) {
  // Вкладку открываем синхронно по клику, иначе popup-блокировщик зарубит window.open после await.
  const openBoard = () => {
    const w = window.open('', '_blank')
    studentApi
      .courseBoardToken(course.id)
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
        <span style={{ fontSize: 14, fontWeight: 600 }}>{course.subject}</span>
        <Button size="sm" onClick={openBoard}>
          Открыть доску
        </Button>
      </div>
    </SectionRow>
  )
}

export default function StudentBoardPage() {
  const { data: courses, isLoading, error, refetch } = useQuery({
    queryKey: ['student-courses'],
    queryFn: () => studentApi.courses(),
  })

  return (
    <div className="space-y-4">
      <SectionCard title="Доска">
        {isLoading && (
          <SectionRow isFirst>
            <span style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>Загрузка...</span>
          </SectionRow>
        )}
        {error != null && !isLoading && (
          <SectionRow isFirst>
            <div className="flex items-center justify-between gap-4">
              <span style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>
                Не удалось загрузить курсы
              </span>
              <Button size="sm" variant="outline" onClick={() => refetch()}>
                Повторить
              </Button>
            </div>
          </SectionRow>
        )}
        {courses && !isLoading && error == null && courses.length === 0 && (
          <EmptyState
            size="sm"
            icon={Presentation}
            title="Курсов пока нет"
            description="У каждого курса своя онлайн-доска: записи с урока, PDF и заметки остаются на ней после занятия. Доски появятся, когда преподаватель добавит тебя на курс"
          />
        )}
        {courses?.map((c, i) => <CourseRow key={c.id} course={c} isFirst={i === 0} />)}
      </SectionCard>
    </div>
  )
}
