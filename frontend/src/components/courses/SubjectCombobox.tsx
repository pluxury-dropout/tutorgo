'use client'

import { useSubjects } from '@/lib/hooks/useCourses'
import {
  Combobox, ComboboxInput, ComboboxContent, ComboboxList, ComboboxItem, ComboboxEmpty,
} from '@/components/ui/combobox'

interface Props {
  value:        string
  onChange:     (subject: string) => void
  placeholder?: string
  autoFocus?:   boolean
}

/** Предмет с подсказками из уже заведённых курсов. Свободный <Input> плодил
 *  «Математику» и «математику» как разные курсы; здесь свой вариант тоже можно
 *  ввести — он просто не подсвечен в списке. */
export function SubjectCombobox({ value, onChange, placeholder = 'Предмет', autoFocus }: Props) {
  const { data: subjects = [] } = useSubjects()
  const typed = value.trim()

  return (
    <Combobox
      items={subjects}
      value={value || null}
      onValueChange={(subject) => onChange(subject ?? '')}
      inputValue={value}
      onInputValueChange={onChange}
    >
      <ComboboxInput placeholder={placeholder} autoFocus={autoFocus} />
      <ComboboxContent>
        <ComboboxEmpty>
          {typed ? `Новый предмет «${typed}»` : 'Предметов пока нет'}
        </ComboboxEmpty>
        <ComboboxList>
          {(subject: string) => (
            <ComboboxItem key={subject} value={subject}>
              {subject}
            </ComboboxItem>
          )}
        </ComboboxList>
      </ComboboxContent>
    </Combobox>
  )
}
