'use client'

import { useState } from 'react'
import { toast } from 'sonner'
import { UserPlus } from 'lucide-react'

import { useStudents, useCreateStudent } from '@/lib/hooks/useStudents'
import {
  Combobox, ComboboxInput, ComboboxContent, ComboboxList, ComboboxItem, ComboboxEmpty,
} from '@/components/ui/combobox'
import type { Student } from '@/types/api'

export function studentName(s: Student): string {
  return [s.first_name, s.last_name].filter(Boolean).join(' ')
}

interface Props {
  value:        Student | null
  onChange:     (student: Student | null) => void
  placeholder?: string
  autoFocus?:   boolean
}

/** Выбор ученика с поиском и созданием на лету: без ученика урок не поставить,
 *  а уходить за ним на другой экран посреди постановки — тупик.
 *  Фильтрация клиентская (список тьютора помещается в память); на > 200
 *  учеников переводить на серверный ?search=. */
export function StudentCombobox({ value, onChange, placeholder = 'Кто?', autoFocus }: Props) {
  const { data: students = [] } = useStudents()
  const createStudent = useCreateStudent()
  const [query, setQuery] = useState('')

  const name = query.trim()

  function handleCreate() {
    if (!name || createStudent.isPending) return
    createStudent.mutate(
      { first_name: name },
      {
        onSuccess: (student) => onChange(student),
        onError:   () => toast.error('Не удалось создать ученика'),
      },
    )
  }

  return (
    <Combobox
      items={students}
      value={value}
      onValueChange={onChange}
      inputValue={query}
      onInputValueChange={setQuery}
      itemToStringLabel={studentName}
    >
      <ComboboxInput placeholder={placeholder} autoFocus={autoFocus} />
      <ComboboxContent>
        <ComboboxEmpty>
          {name ? (
            <button
              type="button"
              onClick={handleCreate}
              className="flex w-full items-center gap-2 rounded-md px-0 py-0 text-left text-foreground hover:underline"
            >
              <UserPlus className="size-3.5 shrink-0" />
              Создать «{name}»
            </button>
          ) : (
            'Учеников пока нет'
          )}
        </ComboboxEmpty>
        <ComboboxList>
          {(student: Student) => (
            <ComboboxItem key={student.id} value={student}>
              {studentName(student)}
            </ComboboxItem>
          )}
        </ComboboxList>
      </ComboboxContent>
    </Combobox>
  )
}
