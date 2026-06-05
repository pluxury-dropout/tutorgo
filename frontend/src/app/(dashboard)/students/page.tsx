'use client'

import { Suspense, useState, useEffect, useRef } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { toast } from 'sonner'
import { Users, Plus, Pencil, Trash2, ChevronRight } from 'lucide-react'

import { useStudentsPaged, useCreateStudent, useUpdateStudent, useDeleteStudent } from '@/lib/hooks/useStudents'
import { StudentForm } from '@/components/students/StudentForm'
import { PageHeader } from '@/components/common/PageHeader'
import { EmptyState } from '@/components/common/EmptyState'
import { Pagination } from '@/components/common/Pagination'
import { StudentFormValues } from '@/schemas/student'
import { Student } from '@/types/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

const LIMIT = 20

function StudentsPageInner() {
  const router      = useRouter()
  const searchParams = useSearchParams()

  const page   = Math.max(1, Number(searchParams.get('page') ?? '1'))
  const search = searchParams.get('search') ?? ''

  const [localSearch, setLocalSearch] = useState(search)
  const mounted = useRef(false)

  // Sync input when URL changes externally (browser back/forward)
  useEffect(() => { setLocalSearch(search) }, [search])

  // Debounce URL write on search input
  useEffect(() => {
    if (!mounted.current) { mounted.current = true; return }
    if (localSearch === search) return
    const t = setTimeout(() => {
      const p = new URLSearchParams()
      if (localSearch) p.set('search', localSearch)
      p.set('page', '1')
      router.replace(`/students?${p}`)
    }, 300)
    return () => clearTimeout(t)
  }, [localSearch]) // eslint-disable-line react-hooks/exhaustive-deps

  function handlePageChange(newPage: number) {
    const p = new URLSearchParams(searchParams.toString())
    p.set('page', String(newPage))
    router.push(`/students?${p}`)
  }

  const { data, isLoading } = useStudentsPaged({ page, limit: LIMIT, search })
  const students   = data?.data ?? []
  const total      = data?.total ?? 0
  const totalPages = Math.ceil(total / LIMIT)

  useEffect(() => {
    if (!isLoading && total > 0 && page > totalPages) {
      handlePageChange(totalPages)
    }
  }, [isLoading, total, page, totalPages]) // eslint-disable-line react-hooks/exhaustive-deps

  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing]   = useState<Student | undefined>()

  const createStudent = useCreateStudent()
  const updateStudent = useUpdateStudent(editing?.id ?? '')
  const deleteStudent = useDeleteStudent()

  function openCreate() { setEditing(undefined); setFormOpen(true) }
  function openEdit(s: Student) { setEditing(s); setFormOpen(true) }

  async function handleSubmit(values: StudentFormValues) {
    if (editing) {
      await updateStudent.mutateAsync(values)
      toast.success('Ученик обновлён')
    } else {
      await createStudent.mutateAsync(values)
      toast.success('Ученик добавлен')
    }
  }

  async function handleDelete(s: Student) {
    if (!confirm(`Удалить ${s.first_name}${s.last_name ? ` ${s.last_name}` : ''}?`)) return
    await deleteStudent.mutateAsync(s.id)
    toast.success('Ученик удалён')
  }

  return (
    <>
      <PageHeader
        title="Ученики"
        description={`${total} учеников`}
        icon={Users}
        iconBg="oklch(0.94 0.03 280)"
        iconColor="oklch(0.42 0.14 280)"
        actions={
          <Button size="sm" onClick={openCreate}>
            <Plus className="h-4 w-4 mr-1.5" /> Добавить
          </Button>
        }
      />

      <div className="mb-4">
        <Input
          placeholder="Поиск по имени или email..."
          value={localSearch}
          onChange={(e) => setLocalSearch(e.target.value)}
          className="max-w-sm"
        />
      </div>

      {isLoading ? (
        <div className="space-y-2">
          {[...Array(5)].map((_, i) => (
            <div key={i} className="h-12 rounded-md bg-muted animate-pulse" />
          ))}
        </div>
      ) : students.length === 0 ? (
        <EmptyState
          icon={Users}
          title={search ? 'Ничего не найдено' : 'Нет учеников'}
          description={search ? 'Попробуй другой запрос' : 'Добавь первого ученика'}
          action={!search ? { label: 'Добавить ученика', onClick: openCreate } : undefined}
        />
      ) : (
        <>
          <div>
            {/* Column headers */}
            <div style={{
              display: 'grid',
              gridTemplateColumns: '1fr 1fr 1fr 16px auto',
              alignItems: 'baseline',
              gap: 12,
              paddingBottom: 8,
              borderBottom: '1px solid var(--border)',
            }}>
              {(['Имя', 'Email', 'Телефон', '', ''] as const).map((label, i) => (
                <span key={i} style={{
                  fontSize: 11.5, fontWeight: 500,
                  color: 'var(--muted-foreground)',
                  letterSpacing: '0.05em',
                  textTransform: 'uppercase',
                }}>{label}</span>
              ))}
            </div>
            {/* Rows */}
            {students.map((student, i) => (
              <div
                key={student.id}
                style={{
                  display: 'grid',
                  gridTemplateColumns: '1fr 1fr 1fr 16px auto',
                  alignItems: 'center',
                  gap: 12,
                  padding: '8px 0',
                  borderTop: i === 0 ? 'none' : '1px solid var(--border)',
                  cursor: 'pointer',
                }}
                className="hover:bg-muted/30 group"
                role="button"
                tabIndex={0}
                onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); router.push(`/students/${student.id}`) } }}
                onClick={() => router.push(`/students/${student.id}`)}
              >
                <span style={{
                  fontSize: 14, fontWeight: 600, color: 'var(--foreground)',
                  overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                }}>
                  {student.first_name}{student.last_name ? ` ${student.last_name}` : ''}
                </span>
                <span style={{ fontSize: 13, color: 'var(--muted-foreground)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {student.email}
                </span>
                <span style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>
                  {student.phone || '—'}
                </span>
                <ChevronRight className="h-4 w-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity duration-150" />
                <div className="flex items-center justify-end gap-1" onClick={(e) => e.stopPropagation()}>
                  <Button size="icon" variant="ghost" className="h-8 w-8" onClick={() => openEdit(student)}>
                    <Pencil className="h-3.5 w-3.5" />
                  </Button>
                  <Button size="icon" variant="ghost"
                    className="h-8 w-8 text-destructive hover:text-destructive"
                    onClick={() => handleDelete(student)}>
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </div>
            ))}
          </div>
          {totalPages > 1 && (
            <div className="flex items-center justify-between mt-3 px-1">
              <span className="text-xs text-muted-foreground">
                Страница {page} из {totalPages}
              </span>
              <Pagination page={page} totalPages={totalPages} onPageChange={handlePageChange} />
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
    </>
  )
}

export default function StudentsPage() {
  return (
    <Suspense>
      <StudentsPageInner />
    </Suspense>
  )
}
