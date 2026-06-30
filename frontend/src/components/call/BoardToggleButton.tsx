'use client'

import { LayoutGrid, LayoutPanelTop } from 'lucide-react'
import { Button } from '@/components/ui/button'

interface BoardToggleButtonProps {
  mode: 'call' | 'board'
  loading: boolean
  onToggle: () => void
}

export function BoardToggleButton({ mode, loading, onToggle }: BoardToggleButtonProps) {
  return (
    <Button
      variant="secondary"
      size="icon"
      onClick={onToggle}
      disabled={loading}
      title={mode === 'call' ? 'Открыть доску' : 'Закрыть доску'}
      aria-label={mode === 'call' ? 'Открыть доску' : 'Закрыть доску'}
      style={{
        position: 'fixed',
        top: 850,
        left: 12,
        zIndex: 9999,
      }}
    >
      {mode === 'call' ? (
        <LayoutPanelTop className="h-4 w-4" />
      ) : (
        <LayoutGrid className="h-4 w-4" />
      )}
    </Button>
  )
}
