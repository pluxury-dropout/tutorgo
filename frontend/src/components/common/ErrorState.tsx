'use client'

import { AlertTriangle } from 'lucide-react'
import { EmptyState } from './EmptyState'

interface ErrorStateProps {
  /** Что не загрузилось, в винительном падеже: «уроки», «платежи», «учеников». */
  what: string
  onRetry?: () => void
  /** sm — для узких карточек дашборда, md — для полноразмерных страниц. */
  size?: 'sm' | 'md'
}

/**
 * Показывается вместо EmptyState, когда запрос упал. Разница принципиальная:
 * EmptyState утверждает «данных нет», и это утверждение о предметной области.
 * Если отрисовать его по ошибке запроса, интерфейс уверенно соврёт — ровно так
 * 500-е на /students и /calendar месяц выглядели как «учеников нет».
 */
export function ErrorState({ what, onRetry, size = 'md' }: ErrorStateProps) {
  return (
    <EmptyState
      size={size}
      icon={AlertTriangle}
      title={`Не удалось загрузить ${what}`}
      description="Сбой связи или ошибка сервера. Это не значит, что данных нет — попробуй ещё раз."
      action={onRetry ? { label: 'Повторить', onClick: onRetry } : undefined}
    />
  )
}
