'use client'

import { useState } from 'react'
import type { ReactNode } from 'react'
import {
  track,
  useEditor,
  DefaultColorStyle,
  DefaultSizeStyle,
  DefaultFontStyle,
  type TLDefaultColorStyle,
  type TLDefaultSizeStyle,
  type TLDefaultFontStyle,
} from '@tldraw/tldraw'
import {
  TOOL_IDS,
  COLORS,
  COLOR_MAP,
  SIZE_KEYS,
  SIZE_MAP,
  SIZE_DOT,
  FONT_DEFS,
  FONT_MAP,
  toEditorTool,
  showColor,
  showThickness,
  showFont,
  hasSettings,
  type UiTool,
} from './boardTools'
import { BoardPageMenu } from './BoardPageMenu'

// Тема paper (константы из мокапа Board.dc.html)
const T = {
  dockInnerBg: '#ffffff',
  dockRadius: 14,
  btnSize: 42,
  btnRadius: 11,
  activeBg: '#26262a',
  activeFg: '#ffffff',
  idleFg: '#4a4a50',
  accent: '#26262a',
  chipActive: '#efece4',
  textMut: '#8c8c93',
  popBg: '#ffffff',
  popRadius: 16,
  dividerColor: 'rgba(0,0,0,.08)',
}

const ICON: Record<UiTool, ReactNode> = {
  select: <path d="M3 3l7.07 16.97 2.51-7.39 7.39-2.51z" />,
  hand: (
    <path d="M18 11V6a2 2 0 0 0-4 0M14 10V4a2 2 0 0 0-4 0v2M10 10.5V6a2 2 0 0 0-4 0v8M18 8a2 2 0 1 1 4 0v6a8 8 0 0 1-8 8h-2c-2.8 0-4.5-.86-5.99-2.34l-3.6-3.6a2 2 0 0 1 2.83-2.82L7 15" />
  ),
  pen: (
    <>
      <path d="M17 3a2.83 2.83 0 0 1 4 4L7.5 20.5 2 22l1.5-5.5z" />
      <path d="M15 5l4 4" />
    </>
  ),
  eraser: (
    <>
      <path d="M7 21l-4.3-4.3a1 1 0 0 1 0-1.4l10-10a1 1 0 0 1 1.4 0l5.6 5.6a1 1 0 0 1 0 1.4L13 21" />
      <path d="M22 21H7M5 11l9 9" />
    </>
  ),
  text: <path d="M4 6.5V4.5h16v2M9 19.5h6M12 4.5v15" />,
  shape: <rect x="4" y="4" width="16" height="16" rx="3" />,
  sticky: (
    <>
      <path d="M15.5 3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h9l6-6V5a2 2 0 0 0-2-2z" />
      <path d="M14 21v-5a2 2 0 0 1 2-2h5" />
    </>
  ),
  image: (
    <>
      <rect x="3" y="3" width="18" height="18" rx="3" />
      <circle cx="8.5" cy="9" r="1.6" />
      <path d="M21 16l-5-5L5 21" />
    </>
  ),
}

const TITLE: Record<UiTool, string> = {
  select: 'Выделение',
  hand: 'Рука',
  pen: 'Перо',
  eraser: 'Ластик',
  text: 'Текст',
  shape: 'Фигура',
  sticky: 'Стикер',
  image: 'Картинка',
}

interface Props {
  onInsertImage: () => void
  isGuest?: boolean
}

export const BoardUi = track(function BoardUi({ onInsertImage, isGuest = false }: Props) {
  const editor = useEditor()
  const [openTool, setOpenTool] = useState<UiTool | null>(null)
  const [pagesOpen, setPagesOpen] = useState(false)

  const active = editor.getCurrentToolId()
  // getSharedStyles отражает стили выделенных фигур; при пустом выделении может
  // быть пусто — подсветку дублируем локальным state, чтобы клик всегда подсвечивался.
  const shared = editor.getSharedStyles()
  const [uiColor, setUiColor] = useState('#26262a')
  const [uiSize, setUiSize] = useState('M')
  const [uiFont, setUiFont] = useState('hand')
  const curColor = shared.getAsKnownValue(DefaultColorStyle) ?? COLOR_MAP[uiColor]
  const curSize = shared.getAsKnownValue(DefaultSizeStyle) ?? SIZE_MAP[uiSize]
  const curFont = shared.getAsKnownValue(DefaultFontStyle) ?? FONT_MAP[uiFont]
  const canUndo = editor.getCanUndo()
  const canRedo = editor.getCanRedo()
  const hasSel = editor.getSelectedShapeIds().length > 0

  function pick(t: UiTool) {
    if (t === 'image') {
      onInsertImage()
      return
    }
    const eid = toEditorTool(t)
    if (eid) editor.setCurrentTool(eid)
    setOpenTool((prev) => (hasSettings(t) ? (prev === t ? null : t) : null))
  }
  function setColor(hex: string) {
    const v = COLOR_MAP[hex] as TLDefaultColorStyle
    setUiColor(hex)
    editor.setStyleForNextShapes(DefaultColorStyle, v)
    editor.setStyleForSelectedShapes(DefaultColorStyle, v)
  }
  function setSize(key: string) {
    const v = SIZE_MAP[key] as TLDefaultSizeStyle
    setUiSize(key)
    editor.setStyleForNextShapes(DefaultSizeStyle, v)
    editor.setStyleForSelectedShapes(DefaultSizeStyle, v)
  }
  function setFont(key: string) {
    const v = FONT_MAP[key] as TLDefaultFontStyle
    setUiFont(key)
    editor.setStyleForNextShapes(DefaultFontStyle, v)
    editor.setStyleForSelectedShapes(DefaultFontStyle, v)
  }

  const iconBtn = (onClick: () => void, disabled: boolean, path: ReactNode) => (
    <button
      onClick={onClick}
      disabled={disabled}
      style={{
        width: 36,
        height: 36,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        border: 'none',
        background: 'transparent',
        borderRadius: 10,
        cursor: disabled ? 'default' : 'pointer',
        color: T.idleFg,
        opacity: disabled ? 0.35 : 1,
      }}
    >
      <svg
        width="19"
        height="19"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.9"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        {path}
      </svg>
    </button>
  )

  return (
    <div
      style={{
        position: 'absolute',
        inset: 0,
        zIndex: 300,
        pointerEvents: 'none',
        fontFamily: "-apple-system,'SF Pro Text',system-ui,'Segoe UI',sans-serif",
        userSelect: 'none',
      }}
    >
      {/* ── Верхний док ── */}
      <div style={{ position: 'absolute', top: 0, left: 0, right: 0, pointerEvents: 'none' }}>
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: '1fr auto 1fr',
            alignItems: 'center',
            padding: '14px 20px',
            pointerEvents: 'none',
          }}
        >
          {/* Левая группа */}
          <div
            style={{
              position: 'relative',
              display: 'flex',
              alignItems: 'center',
              gap: 4,
              justifySelf: 'start',
              pointerEvents: 'auto',
            }}
          >
            {iconBtn(() => setPagesOpen((v) => !v), false, <path d="M4 7h16M4 12h16M4 17h16" />)}
            <button
              onClick={() => setPagesOpen((v) => !v)}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 6,
                border: 'none',
                background: 'transparent',
                borderRadius: 11,
                cursor: 'pointer',
                color: T.accent,
                fontSize: 13,
                fontWeight: 600,
                padding: '7px 11px',
              }}
            >
              Урок
              <svg
                width="13"
                height="13"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <path d="M6 9l6 6 6-6" />
              </svg>
            </button>
            <div style={{ width: 1, height: 20, background: 'rgba(0,0,0,.10)', margin: '0 5px' }} />
            {iconBtn(
              () => editor.undo(),
              !canUndo,
              <>
                <path d="M9 14l-4-4 4-4" />
                <path d="M5 10h11a4 4 0 0 1 0 8h-3" />
              </>,
            )}
            {iconBtn(
              () => editor.redo(),
              !canRedo,
              <>
                <path d="M15 14l4-4-4-4" />
                <path d="M19 10H8a4 4 0 0 0 0 8h3" />
              </>,
            )}
            {pagesOpen && (
              <div
                style={{ position: 'absolute', top: 44, left: 0, pointerEvents: 'auto' }}
                onPointerDown={(e) => e.stopPropagation()}
              >
                <BoardPageMenu />
              </div>
            )}
          </div>

          {/* Центр: инструменты */}
          <div
            style={{
              justifySelf: 'center',
              display: 'flex',
              alignItems: 'center',
              gap: 4,
              padding: 6,
              background: T.dockInnerBg,
              borderRadius: T.dockRadius,
              boxShadow: '0 4px 16px rgba(30,30,34,0.10)',
              pointerEvents: 'auto',
            }}
          >
            {TOOL_IDS.filter((t) => !(isGuest && t === 'image')).map((t) => {
              const isActive = t !== 'image' && toEditorTool(t) === active
              return (
                <button
                  key={t}
                  onClick={() => pick(t)}
                  title={TITLE[t]}
                  style={{
                    width: T.btnSize,
                    height: T.btnSize,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    border: 'none',
                    cursor: 'pointer',
                    transition: 'background .15s,color .15s',
                    borderRadius: T.btnRadius,
                    background: isActive ? T.activeBg : 'transparent',
                    color: isActive ? T.activeFg : T.idleFg,
                  }}
                >
                  <svg
                    width="20"
                    height="20"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="1.8"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    {ICON[t]}
                  </svg>
                </button>
              )
            })}
          </div>

          {/* Правая группа */}
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 4,
              justifySelf: 'end',
              pointerEvents: 'auto',
            }}
          >
            {iconBtn(
              () => editor.deleteShapes(editor.getSelectedShapeIds()),
              !hasSel,
              <path d="M5 7h14M10 7V5h4v2M7 7l1 12h8l1-12" />,
            )}
            {/* ponytail: ⋮ — заглушка, add when needed */}
            {iconBtn(
              () => {},
              false,
              <>
                <circle cx="12" cy="5" r="1.5" />
                <circle cx="12" cy="12" r="1.5" />
                <circle cx="12" cy="19" r="1.5" />
              </>,
            )}
          </div>
        </div>

        {/* ── Поповер настроек ── */}
        {openTool && hasSettings(openTool) && (
          <div
            style={{
              position: 'absolute',
              left: '50%',
              top: 'calc(100% + 10px)',
              transform: 'translateX(-50%)',
              display: 'flex',
              alignItems: 'center',
              gap: 12,
              background: T.popBg,
              border: '1px solid rgba(0,0,0,0.06)',
              boxShadow: '0 14px 36px rgba(40,36,28,0.18)',
              borderRadius: T.popRadius,
              padding: '9px 14px',
              pointerEvents: 'auto',
            }}
          >
            {showColor(openTool) && (
              <div style={{ display: 'flex', alignItems: 'center', gap: 7 }}>
                {COLORS.map((hex) => {
                  const sel = curColor === COLOR_MAP[hex]
                  return (
                    <button
                      key={hex}
                      onClick={() => setColor(hex)}
                      style={{
                        width: 21,
                        height: 21,
                        borderRadius: '50%',
                        border: 'none',
                        cursor: 'pointer',
                        background: hex,
                        boxShadow: sel
                          ? `0 0 0 2px ${T.popBg},0 0 0 4px ${T.accent}`
                          : 'inset 0 0 0 1px rgba(0,0,0,0.10)',
                      }}
                    />
                  )
                })}
              </div>
            )}
            {showColor(openTool) && showThickness(openTool) && (
              <div style={{ width: 1, height: 24, background: T.dividerColor }} />
            )}
            {showThickness(openTool) && (
              <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                {SIZE_KEYS.map((k) => {
                  const sel = curSize === SIZE_MAP[k]
                  return (
                    <button
                      key={k}
                      onClick={() => setSize(k)}
                      style={{
                        width: 30,
                        height: 30,
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        border: 'none',
                        borderRadius: 9,
                        cursor: 'pointer',
                        background: sel ? T.chipActive : 'transparent',
                      }}
                    >
                      <span
                        style={{
                          display: 'block',
                          width: SIZE_DOT[k],
                          height: SIZE_DOT[k],
                          borderRadius: '50%',
                          background: sel ? T.accent : '#a3a3a9',
                        }}
                      />
                    </button>
                  )
                })}
              </div>
            )}
            {showFont(openTool) && (
              <>
                <div style={{ width: 1, height: 24, background: T.dividerColor }} />
                <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                  {FONT_DEFS.map((f) => {
                    const sel = curFont === FONT_MAP[f.key]
                    return (
                      <button
                        key={f.key}
                        onClick={() => setFont(f.key)}
                        style={{
                          minWidth: 36,
                          height: 30,
                          padding: '0 9px',
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                          border: 'none',
                          borderRadius: 9,
                          cursor: 'pointer',
                          fontSize: 15,
                          fontFamily: f.family,
                          background: sel ? T.chipActive : 'transparent',
                          color: sel ? T.accent : T.textMut,
                        }}
                      >
                        {f.label}
                      </button>
                    )
                  })}
                </div>
                <div style={{ width: 1, height: 24, background: T.dividerColor }} />
                <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                  {SIZE_KEYS.map((k) => {
                    const sel = curSize === SIZE_MAP[k]
                    return (
                      <button
                        key={k}
                        onClick={() => setSize(k)}
                        style={{
                          width: 32,
                          height: 30,
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                          border: 'none',
                          borderRadius: 9,
                          cursor: 'pointer',
                          fontSize: 11,
                          fontWeight: 600,
                          background: sel ? T.chipActive : 'transparent',
                          color: sel ? T.accent : T.textMut,
                        }}
                      >
                        {k}
                      </button>
                    )
                  })}
                </div>
              </>
            )}
          </div>
        )}
      </div>
    </div>
  )
})
