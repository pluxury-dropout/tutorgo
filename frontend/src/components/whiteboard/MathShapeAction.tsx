'use client'

import { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'

/**
 * Кнопка внутри панели свойств Excalidraw.
 *
 * Официальной точки расширения нет: UIOptions знает только canvasActions и
 * tools.image, а панель зовёт renderAction по фиксированным именам. Поэтому —
 * портал в живой DOM.
 *
 * Цель — .panelColumn, а не .App-menu__left: первый существует и в мобильной
 * вёрстке. Контейнер размонтируется при снятии выделения, и React создаёт
 * НОВЫЙ узел, поэтому цель переопрашивается по сигналу извне (onChange доски)
 * плюс MutationObserver как подстраховка.
 */
interface Props {
  /** меняется при каждом onChange доски — повод переспросить контейнер */
  revision: number
  container: HTMLElement | null
  mode: 'text' | 'formula' | null
  onConvert: () => void
  onEdit: () => void
  onToText: () => void
}

export function MathShapeAction({ revision, container, mode, onConvert, onEdit, onToText }: Props) {
  const [target, setTarget] = useState<HTMLElement | null>(null)

  useEffect(() => {
    if (!container) return
    const find = () => setTarget(container.querySelector<HTMLElement>('.panelColumn'))
    find()
    const observer = new MutationObserver(find)
    observer.observe(container, { childList: true, subtree: true })
    return () => observer.disconnect()
  }, [container, revision])

  if (!target || !mode) return null

  return createPortal(
    <div style={{ order: -1, display: 'flex', gap: 6, paddingBottom: 4 }}>
      {mode === 'text' ? (
        <button type="button" onClick={onConvert} style={buttonStyle}>
          ∑ В формулу
        </button>
      ) : (
        <>
          <button type="button" onClick={onEdit} style={buttonStyle}>
            ∑ Изменить
          </button>
          <button type="button" onClick={onToText} style={buttonStyle}>
            В текст
          </button>
        </>
      )}
    </div>,
    target
  )
}

const buttonStyle: React.CSSProperties = {
  padding: '4px 8px',
  fontSize: 12,
  borderRadius: 6,
  border: '1px solid var(--border)',
  background: 'var(--card)',
  color: 'var(--foreground)',
  cursor: 'pointer',
}
