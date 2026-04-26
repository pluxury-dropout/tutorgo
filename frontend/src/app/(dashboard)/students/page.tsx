'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { toast } from 'sonner'
import { Users, Plus, Pencil, Trash2 } from 'lucide-react'

import { useStudents, useCreateStudent, useUpdateStudent, useDeleteStudent } from '@/lib/hooks/useStudents'
import { StudentForm } from '@/components/students/StudentForm'
import { PageHeader } from '@/components/common/PageHeader'
import { EmptyState } from '@/components/common/EmptyState'
import { StudentFormValues } from '@/schemas/student'
import { Student } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

export default function StudentsPage() {
  const router = useRouter()
  const { data: students = [], isLoading } = useStudents()

  const [search, setSearch]   = useState('')
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing]   = useState<Student | undefined>()

  const createStudent = useCreateStudent()
  const updateStudent = useUpdateStudent(editing?.id ?? '')
  const deleteStudent = useDeleteStudent()

  
  const filtered: Student[] = students.filter((student) => {
    const q = search.toLowerCase()
    return (
      student.first_name.toLowerCase().includes(q) ||
      (student.last_name?.toLowerCase().includes(q) ?? false) ||
      student.email.toLowerCase().includes(q)
    )
  })

  function openCreate() {
    setEditing(undefined)
    setFormOpen(true)
  }

  function openEdit(student: Student) {
    setEditing(student)
    setFormOpen(true)
  }

  async function handleSubmit(values: StudentFormValues) {
    if (editing) {
      await updateStudent.mutateAsync(values)
      toast.success('Ученик обновлён')
    } else {
      await createStudent.mutateAsync(values)
      toast.success('Ученик добавлен')
    }
  }

  async function handleDelete(student: Student) {
    if (!confirm(`Удалить ${student.first_name}${student.last_name ? ` ${student.last_name}` : ''}?`)) return
    await deleteStudent.mutateAsync(student.id)
    toast.success('Ученик удалён')
  }

  return (
    <>
      <PageHeader
        title="Ученики"
        description={`${students.length} учеников`}
        actions={
          <Button size="sm" onClick={openCreate}>
            <Plus className="h-4 w-4 mr-1.5" /> Добавить
          </Button>
        }
      />

      <div className="mb-4">
        <Input
          placeholder="Поиск по имени или email..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-sm"
        />
      </div>

      {isLoading ? (
        <div className="space-y-2">
          {[...Array(5)].map((_, i) => (
            <div key={i} className="h-12 rounded-md bg-muted animate-pulse" />
          ))}
        </div>
      ) : filtered.length === 0 ? (
        <EmptyState
          icon={Users}
          title={search ? 'Ничего не найдено' : 'Нет учеников'}
          description={search ? 'Попробуй другой запрос' : 'Добавь первого ученика'}
          action={!search ? { label: 'Добавить ученика', onClick: openCreate } : undefined}
        />
      ) : (
        <div className="border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/40">
                <th className="text-left px-4 py-3 font-medium text-muted-foreground">Имя</th>
                <th className="text-left px-4 py-3 font-medium text-muted-foreground">Email</th>
                <th className="text-left px-4 py-3 font-medium text-muted-foreground">Телефон</th>
                <th className="px-4 py-3" />
              </tr>
            </thead>
            <tbody>
              {filtered.map((student) => (
                <tr
                  key={student.id}
                  className="border-b last:border-0 hover:bg-muted/30 cursor-pointer"
                  onClick={() => router.push(`/students/${student.id}`)}
                >
                  <td className="px-4 py-3 font-medium">
                    {student.first_name}{student.last_name ? ` ${student.last_name}` : ''}
                  </td>
                  <td className="px-4 py-3 text-muted-foreground">{student.email}</td>
                  <td className="px-4 py-3 text-muted-foreground">{student.phone || '—'}</td>
                  <td className="px-4 py-3">
                    <div
                      className="flex items-center justify-end gap-1"
                      onClick={(e) => e.stopPropagation()}
                    >
                      <Button size="icon" variant="ghost" className="h-8 w-8"
                        onClick={() => openEdit(student)}>
                        <Pencil className="h-3.5 w-3.5" />
                      </Button>
                      <Button size="icon" variant="ghost"
                        className="h-8 w-8 text-destructive hover:text-destructive"
                        onClick={() => handleDelete(student)}>
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
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
