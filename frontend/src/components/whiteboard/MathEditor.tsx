'use client'

import { useEffect, useRef } from 'react'
import 'mathlive/fonts.css'

// Живой набор: ученик видит формулу по мере ввода. Не троттл, а debounce с
// потолком — Excalidraw НИКОГДА не удаляет файлы из памяти, а каждый кадр
// набора порождает новый fileId. Посимвольная трансляция оставила бы сотню
// SVG на формулу у каждого участника.
const DRAFT_DEBOUNCE_MS = 250
const DRAFT_MAX_WAIT_MS = 1000

interface Props {
  initialLatex: string
  /** промежуточный кадр — уезжает пирам, в историю не пишется */
  onDraft: (latex: string) => void
  /** финал: Enter или кнопка «Готово» */
  onCommit: (latex: string) => void
  onCancel: () => void
  /** экранные координаты левого-нижнего угла формулы */
  anchor: { left: number; top: number }
}

export function MathEditor({ initialLatex, onDraft, onCommit, onCancel, anchor }: Props) {
  const hostRef = useRef<HTMLDivElement>(null)
  // Колбэки в ref: эффект монтирует поле один раз, пересоздавать его на каждый
  // ре-рендер родителя нельзя — потеряется каретка и фокус. Запись — в эффекте
  // без зависимостей (после каждого рендера), а не в теле рендера: react-hooks/refs
  // запрещает мутировать ref во время рендера.
  const cbRef = useRef({ onDraft, onCommit, onCancel })
  useEffect(() => {
    cbRef.current = { onDraft, onCommit, onCancel }
  })

  useEffect(() => {
    let disposed = false
    let timer: ReturnType<typeof setTimeout> | null = null
    let firstEditAt = 0
    let field: HTMLElement | null = null

    void (async () => {
      // Статический импорт нельзя: при SSR condition "node" отдаёт сборку без
      // MathfieldElement, и он молча оказывается undefined.
      const { MathfieldElement } = await import('mathlive')
      if (disposed || !hostRef.current) return

      // Ни одного сетевого запроса: шрифты уже пришли из fonts.css, звуки не нужны.
      MathfieldElement.fontsDirectory = null
      MathfieldElement.soundsDirectory = null

      const mf = new MathfieldElement()
      field = mf
      mf.value = initialLatex
      // На доске рядом стилус — автопоказ клавиатуры на touch мешал бы.
      mf.mathVirtualKeyboardPolicy = 'manual'
      mf.style.cssText = 'min-width:320px;font-size:20px;padding:6px 8px;border:none;outline:none;background:transparent;color:var(--foreground)'

      const flush = () => {
        if (timer) clearTimeout(timer)
        timer = null
        firstEditAt = 0
        cbRef.current.onDraft(mf.value)
      }

      mf.addEventListener('input', () => {
        if (!firstEditAt) firstEditAt = Date.now()
        if (Date.now() - firstEditAt >= DRAFT_MAX_WAIT_MS) {
          flush()
          return
        }
        if (timer) clearTimeout(timer)
        timer = setTimeout(flush, DRAFT_DEBOUNCE_MS)
      })

      mf.addEventListener('keydown', (e) => {
        if (e.key === 'Enter') {
          e.preventDefault()
          cbRef.current.onCommit(mf.value)
        }
        if (e.key === 'Escape') {
          e.preventDefault()
          cbRef.current.onCancel()
        }
      })

      hostRef.current.append(mf)
      mf.focus()
    })()

    return () => {
      disposed = true
      if (timer) clearTimeout(timer)
      field?.remove()
    }
  }, [initialLatex])

  return (
    <div
      data-board-ui
      style={{
        position: 'absolute',
        left: anchor.left,
        top: anchor.top,
        zIndex: 6,
        display: 'flex',
        alignItems: 'center',
        gap: 8,
        padding: 6,
        borderRadius: 10,
        border: '1px solid var(--border)',
        background: 'var(--card)',
        boxShadow: '0 8px 24px rgb(0 0 0 / 0.12)',
      }}
      // Клики по редактору не должны уходить в холст и снимать выделение.
      onPointerDown={(e) => e.stopPropagation()}
    >
      <div ref={hostRef} />
      <button
        type="button"
        onClick={() => window.mathVirtualKeyboard.show()}
        style={{ padding: '4px 8px', fontSize: 13, cursor: 'pointer' }}
      >
        Символы
      </button>
    </div>
  )
}
