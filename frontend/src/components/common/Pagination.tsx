'use client'

import { ChevronLeft, ChevronRight } from 'lucide-react'
import { Button } from '@/components/ui/button'

interface PaginationProps {
  page: number
  totalPages: number
  onPageChange: (page: number) => void
}

function pageNumbers(page: number, total: number): (number | '…')[] {
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1)
  if (page <= 4) return [1, 2, 3, 4, 5, '…', total]
  if (page >= total - 3) return [1, '…', total - 4, total - 3, total - 2, total - 1, total]
  return [1, '…', page - 1, page, page + 1, '…', total]
}

export function Pagination({ page, totalPages, onPageChange }: PaginationProps) {
  if (totalPages <= 1) return null
  const pages = pageNumbers(page, totalPages)
  return (
    <div className="flex items-center gap-1">
      <Button
        size="icon"
        variant="ghost"
        className="h-8 w-8"
        disabled={page <= 1}
        onClick={() => onPageChange(page - 1)}
      >
        <ChevronLeft className="h-4 w-4" />
      </Button>
      {pages.map((p, i) =>
        p === '…' ? (
          <span key={`e${i}`} className="px-2 text-sm text-muted-foreground">
            …
          </span>
        ) : (
          <Button
            key={p}
            size="icon"
            variant={p === page ? 'default' : 'ghost'}
            className="h-8 w-8 text-sm"
            onClick={() => onPageChange(p as number)}
          >
            {p}
          </Button>
        )
      )}
      <Button
        size="icon"
        variant="ghost"
        className="h-8 w-8"
        disabled={page >= totalPages}
        onClick={() => onPageChange(page + 1)}
      >
        <ChevronRight className="h-4 w-4" />
      </Button>
    </div>
  )
}
