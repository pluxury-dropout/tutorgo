'use client'

import { useRouter } from 'next/navigation'
import { useQuery } from '@tanstack/react-query'
import { toast } from 'sonner'

import { studentApi } from '@/lib/api/student'
import { SectionCard, SectionRow } from '@/components/common/SectionCard'
import { Button } from '@/components/ui/button'
import type { StudentCourse } from '@/types/api'

function CourseRow({ course, isFirst }: { course: StudentCourse; isFirst: boolean }) {
  const router = useRouter()

  const openBoard = async () => {
    try {
      const { invite_token } = await studentApi.courseBoardToken(course.id)
      router.push(`/board/join/${invite_token}`)
    } catch {
      toast.error('Не удалось открыть доску')
    }
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
          <SectionRow isFirst>
            <span style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>Курсов пока нет</span>
          </SectionRow>
        )}
        {courses?.map((c, i) => <CourseRow key={c.id} course={c} isFirst={i === 0} />)}
      </SectionCard>
    </div>
  )
}
