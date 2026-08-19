'use client'

import { useEffect, useRef, useState } from 'react'
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
  /** финал: Enter */
  onCommit: (latex: string) => void
  onCancel: () => void
  /** экранные координаты левого-нижнего угла формулы */
  anchor: { left: number; top: number }
}

export function MathEditor({ initialLatex, onDraft, onCommit, onCancel, anchor }: Props) {
  const hostRef = useRef<HTMLDivElement>(null)
  // Высота открытой виртуальной клавиатуры (0 — закрыта). Она fixed по низу
  // окна, а редактор по умолчанию стоит внизу доски — то есть ровно под ней.
  const [kbdHeight, setKbdHeight] = useState(0)
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
    let unwatchKbd: (() => void) | null = null

    void (async () => {
      // Статический импорт нельзя: при SSR condition "node" отдаёт сборку без
      // MathfieldElement, и он молча оказывается undefined.
      let mathlive: typeof import('mathlive')
      try {
        mathlive = await import('mathlive')
      } catch (err) {
        // Сбой загрузки чанка (сеть, блокировщик) — редактор остаётся без
        // поля, но без необработанного отказа промиса. Тот же приём, что и
        // в useMathFiles.ts для соседнего динамического импорта.
        console.warn('Не удалось загрузить редактор формул', err)
        return
      }
      const { MathfieldElement } = mathlive
      if (disposed || !hostRef.current) return

      // Ни одного сетевого запроса: шрифты уже пришли из fonts.css, звуки не нужны.
      MathfieldElement.fontsDirectory = null
      MathfieldElement.soundsDirectory = null

      // Клавиша «спрятать клавиатуру» есть только в раскладках compact/minimalist;
      // в дефолтных (numeric/symbols/alphabetic/greek) её нет, а закрытие по потере
      // фокуса MathLive делает только при policy != 'manual'. Оставался единственный
      // выход — тумблер внутри поля, который сама же клавиатура и закрывает собой
      // (она fixed внизу экрана, поле — тоже внизу). Переопределяем [action]: ⏎
      // шлёт commit, событие которого мы не слушаем, то есть кнопка была мёртвой.
      const kbd = window.mathVirtualKeyboard
      kbd.setKeycap('[action]', {
        class: 'action',
        command: ['hideVirtualKeyboard'],
        width: 1.5,
        label: '<svg class=svg-glyph-lg><use xlink:href=#svg-keyboard-down /></svg>',
      })

      // Клавиатура открыта — редактор паркуется прямо над ней, иначе набирать
      // пришлось бы вслепую.
      const syncKbd = () => setKbdHeight(kbd.visible ? kbd.boundingRect.height : 0)
      kbd.addEventListener('virtual-keyboard-toggle', syncKbd)
      kbd.addEventListener('geometrychange', syncKbd)
      unwatchKbd = () => {
        kbd.removeEventListener('virtual-keyboard-toggle', syncKbd)
        kbd.removeEventListener('geometrychange', syncKbd)
      }

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
        // Сток ввода MathLive всплывает до document с ретаргетом на
        // <math-field>; Excalidraw слушает document keydown безусловно и не
        // распознаёт contenteditable-сток как текстовый ввод (isWritableElement
        // не знает про него). Без stopPropagation любая буква — это шорткат
        // инструмента, Backspace/Delete удаляет выделенную формулу, Enter
        // включает обрезку картинки. stopPropagation (не Immediate) — наш же
        // обработчик Enter/Esc ниже должен отработать как обычно.
        e.stopPropagation()
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
      unwatchKbd?.()
      // Клавиатура — глобальный синглтон на body: без этого она переживает
      // редактор (и оставляет за собой padding-bottom у body), а закрыть её
      // уже нечем — поля с тумблером на экране больше нет.
      if ('mathVirtualKeyboard' in window) window.mathVirtualKeyboard.hide()
    }
  }, [initialLatex])

  return (
    <div
      data-board-ui
      style={{
        // z-index клавиатуры — 105, так что в припаркованном виде надо выше.
        ...(kbdHeight
          ? { position: 'fixed' as const, left: '50%', transform: 'translateX(-50%)', bottom: kbdHeight + 12, zIndex: 110 }
          : { position: 'absolute' as const, left: anchor.left, top: anchor.top, zIndex: 6 }),
        display: 'flex',
        alignItems: 'center',
        padding: 6,
        borderRadius: 10,
        border: '1px solid var(--border)',
        background: 'var(--card)',
        boxShadow: '0 8px 24px rgb(0 0 0 / 0.12)',
      }}
      // Клики по редактору не должны уходить в холст и снимать выделение.
      onPointerDown={(e) => e.stopPropagation()}
    >
      {/* Кнопки клавиатуры тут нет: MathLive рисует свой тумблер внутри поля,
          и он умеет не только показать её, но и спрятать. */}
      <div ref={hostRef} />
    </div>
  )
}
