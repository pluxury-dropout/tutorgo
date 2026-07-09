'use client'

import { useRouter } from 'next/navigation'
import { Pencil, Trash2, ChevronRight } from 'lucide-react'
import { SectionCard } from '@/components/common/SectionCard'
import { Button } from '@/components/ui/button'
import { Student } from '@/types/api'

interface StudentsListProps {
  students: Student[]
  onEdit: (s: Student) => void
  onDelete: (s: Student) => void
}

export function StudentsList({ students, onEdit, onDelete }: StudentsListProps) {
  const router = useRouter()

  return (
    <SectionCard>
      {students.map((student, i) => (
        <div
          key={student.id}
          style={{
            display: 'grid',
            gridTemplateColumns: 'minmax(0, 1fr) auto',
            alignItems: 'center',
            gap: 16,
            padding: '10px 18px',
            borderTop: i === 0 ? 'none' : '1px solid var(--row-border)',
            cursor: 'pointer',
          }}
          className="hover:bg-muted/30 group"
          role="button"
          tabIndex={0}
          onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); router.push(`/students/${student.id}`) } }}
          onClick={() => router.push(`/students/${student.id}`)}
        >
          <div style={{ minWidth: 0 }}>
            <div style={{
              fontSize: 14, fontWeight: 600, color: 'var(--foreground)',
              overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
            }}>
              {student.first_name}{student.last_name ? ` ${student.last_name}` : ''}
            </div>
            {student.email && (
              <div style={{
                fontSize: 12.5, color: 'var(--muted-foreground)', marginTop: 1,
                overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
              }}>
                {student.email}
              </div>
            )}
          </div>
          <div className="flex items-center gap-3 shrink-0">
            <span style={{ fontSize: 13, color: 'var(--muted-foreground)', fontVariantNumeric: 'tabular-nums' }}>
              {student.phone || '—'}
            </span>
            <ChevronRight className="h-4 w-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity duration-150" />
            <div className="flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
              <Button size="icon" variant="ghost" className="h-8 w-8" onClick={() => onEdit(student)}>
                <Pencil className="h-3.5 w-3.5" />
              </Button>
              <Button size="icon" variant="ghost"
                className="h-8 w-8 text-destructive hover:text-destructive"
                onClick={() => onDelete(student)}>
                <Trash2 className="h-3.5 w-3.5" />
              </Button>
            </div>
          </div>
        </div>
      ))}
    </SectionCard>
  )
}
