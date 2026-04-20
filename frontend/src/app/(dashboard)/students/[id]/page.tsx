'use client'

import { useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { toast } from 'sonner'
import { ArrowLeft, Pencil, Trash2 } from 'lucide-react'

import { useStudent, useUpdateStudent, useDeleteStudent } from '@/lib/hooks/useStudents'
import { StudentForm } from '@/components/students/StudentForm'
import { PageHeader } from '@/components/common/PageHeader'
import { StudentFormValues } from '@/schemas/student'

import { Button } from '@/components/ui/button'

export default function StudentDetailPage() {
  const { id } = useParams<{ id: string }>()
  const router  = useRouter()

  const { data: student, isLoading } = useStudent(id)
  const updateStudent = useUpdateStudent(id)
  const deleteStudent = useDeleteStudent()

  const [formOpen, setFormOpen] = useState(false)

  async function handleUpdate(values: StudentFormValues) {
    await updateStudent.mutateAsync(values)
    toast.success('Ученик обновлён')
  }

  async function handleDelete() {
    if (!student) return
    if (!confirm(`Удалить ${student.first_name} ${student.last_name}?`)) return
    await deleteStudent.mutateAsync(student.id)
    toast.success('Ученик удалён')
    router.push('/students')
  }

  if (isLoading) {
    return <div className="h-32 rounded-lg bg-muted animate-pulse" />
  }

  if (!student) {
    return <p className="text-sm text-muted-foreground">Ученик не найден</p>
  }

  return (
    <>
      <button
        onClick={() => router.push('/students')}
        className="flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground mb-4"
      >
        <ArrowLeft className="h-4 w-4" /> Все ученики
      </button>

      <PageHeader
        title={`${student.first_name} ${student.last_name}`}
        actions={
          <div className="flex gap-2">
            <Button size="sm" variant="outline" onClick={() => setFormOpen(true)}>
              <Pencil className="h-4 w-4 mr-1.5" /> Редактировать
            </Button>
            <Button size="sm" variant="destructive" onClick={handleDelete}>
              <Trash2 className="h-4 w-4 mr-1.5" /> Удалить
            </Button>
          </div>
        }
      />

      <div className="border rounded-lg bg-card p-5 max-w-md space-y-3">
        <Row label="Email"   value={student.email} />
        <Row label="Телефон" value={student.phone || '—'} />
      </div>

      <StudentForm
        open={formOpen}
        onClose={() => setFormOpen(false)}
        onSubmit={handleUpdate}
        initial={student}
      />
    </>
  )
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center gap-4">
      <span className="text-sm text-muted-foreground w-20 shrink-0">{label}</span>
      <span className="text-sm font-medium">{value}</span>
    </div>
  )
}
