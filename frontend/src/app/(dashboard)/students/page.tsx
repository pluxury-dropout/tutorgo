'use client'

import { useState } from 'react'
import { toast } from 'sonner'
import { Plus, Users } from 'lucide-react'

import { useStudentsPaged, useUpdateStudent, useRemoveStudent, useRestoreStudent } from '@/lib/hooks/useStudents'
import { StudentForm } from '@/components/students/StudentForm'
import { StudentOnboardingDialog } from '@/components/students/StudentOnboardingDialog'
import { StudentsList } from '@/components/students/StudentsList'
import { PageHeader, HeaderMetric } from '@/components/common/PageHeader'
import { EmptyState } from '@/components/common/EmptyState'
import { ErrorState } from '@/components/common/ErrorState'
import { Pagination } from '@/components/common/Pagination'
import { StudentFormValues } from '@/schemas/student'
import { Student } from '@/types/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

const LIMIT = 20

/** Ученики — свой раздел, а не вкладка курсов: для репетитора первичен человек,
 *  курс у него производный. */
export default function StudentsPage() {
  const [search, setSearch] = useState('')
  const [page, setPage]     = useState(1)
  const [archived, setArchived] = useState(false)

  // Сброс страницы делаем в обработчике, а не эффектом на search: иначе после
  // каждой буквы идёт лишний рендер со старым номером страницы.
  function handleSearch(value: string) {
    setSearch(value)
    setPage(1)
  }

  function switchArchived(value: boolean) {
    setArchived(value)
    setPage(1)
  }

  const { data, isLoading, isError, refetch } = useStudentsPaged({ page, limit: LIMIT, search, archived })
  const students   = data?.data ?? []
  const total      = data?.total ?? 0
  const totalPages = Math.ceil(total / LIMIT)

  const [formOpen, setFormOpen]       = useState(false)
  const [onboardOpen, setOnboardOpen] = useState(false)
  const [editing, setEditing]         = useState<Student | undefined>()

  const updateStudent = useUpdateStudent(editing?.id ?? '')
  const removeStudent  = useRemoveStudent()
  const restoreStudent = useRestoreStudent()

  function openEdit(s: Student) { setEditing(s); setFormOpen(true) }

  // Создание переехало в StudentOnboardingDialog (спека 8.2, фаза 4) — за один
  // сабмит заводит ещё и курс с расписанием. StudentForm остался только для
  // правки контактов уже заведённого ученика.
  async function handleSubmit(values: StudentFormValues) {
    await updateStudent.mutateAsync(values)
    toast.success('Ученик обновлён')
  }

  async function handleDelete(s: Student) {
    const result = await removeStudent(s)
    if (result === 'deleted')  toast.success('Ученик удалён')
    if (result === 'archived') toast.success('Ученик перенесён в архив')
  }

  async function handleRestore(s: Student) {
    await restoreStudent.mutateAsync(s.id)
    toast.success('Ученик восстановлен')
  }

  return (
    <div style={{ maxWidth: 900 }}>
      <PageHeader
        title="Ученики"
        meta={<HeaderMetric color="var(--purple)">{total} {archived ? 'в архиве' : 'учеников'}</HeaderMetric>}
        actions={
          <Button size="sm" onClick={() => setOnboardOpen(true)}>
            <Plus className="h-4 w-4 mr-1.5" /> Добавить
          </Button>
        }
      />

      <div className="mb-4 flex flex-wrap items-center gap-2">
        <Input
          placeholder="Поиск по имени или email..."
          value={search}
          onChange={(e) => handleSearch(e.target.value)}
          className="max-w-sm"
        />
        <Button size="sm" variant={archived ? 'outline' : 'secondary'} onClick={() => switchArchived(false)}>
          Активные
        </Button>
        <Button size="sm" variant={archived ? 'secondary' : 'outline'} onClick={() => switchArchived(true)}>
          Архив
        </Button>
      </div>

      {isLoading ? (
        <div className="space-y-2">
          {[...Array(5)].map((_, i) => (
            <div key={i} className="h-12 rounded-md bg-muted animate-pulse" />
          ))}
        </div>
      ) : isError ? (
        <ErrorState what="учеников" onRetry={() => refetch()} />
      ) : students.length === 0 ? (
        <EmptyState
          icon={Users}
          title={search ? 'Ничего не найдено' : archived ? 'Архив пуст' : 'Учеников пока нет'}
          description={search
            ? 'Попробуй другой запрос — поиск идёт по имени и контактам'
            : archived
              ? 'Сюда попадают ученики с платежами или проведёнными уроками, когда их удаляют: история оплат остаётся'
              : 'Ученик — карточка с контактами. К ней привязываются курсы, уроки и оплаты, а сам ученик может получить доступ в личный кабинет'}
          action={!search && !archived ? { label: 'Добавить ученика', onClick: () => setOnboardOpen(true) } : undefined}
        />
      ) : (
        <>
          <StudentsList
            students={students}
            onEdit={openEdit}
            onDelete={handleDelete}
            onRestore={archived ? handleRestore : undefined}
          />
          {totalPages > 1 && (
            <div className="flex items-center justify-between mt-3 px-1">
              <span className="text-xs text-muted-foreground">
                Страница {page} из {totalPages}
              </span>
              <Pagination page={page} totalPages={totalPages} onPageChange={setPage} />
            </div>
          )}
        </>
      )}

      <StudentForm
        open={formOpen}
        onClose={() => setFormOpen(false)}
        onSubmit={handleSubmit}
        initial={editing}
      />
      <StudentOnboardingDialog open={onboardOpen} onClose={() => setOnboardOpen(false)} />
    </div>
  )
}
