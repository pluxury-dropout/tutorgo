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
      size="sm"
      onClick={onToggle}
      disabled={loading}
      style={{
        position: 'absolute',
        top: 12,
        left: 12,
        zIndex: 50,
      }}
    >
      {mode === 'call' ? (
        <>
          <LayoutPanelTop className="h-4 w-4 mr-2" />
          Открыть доску
        </>
      ) : (
        <>
          <LayoutGrid className="h-4 w-4 mr-2" />
          Закрыть доску
        </>
      )}
    </Button>
  )
}
