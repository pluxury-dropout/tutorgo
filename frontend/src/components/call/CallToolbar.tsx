// frontend/src/components/call/CallToolbar.tsx
'use client'

import { useEffect, useRef, useState, type ReactNode } from 'react'
import { useLocalParticipant } from '@livekit/components-react'
import { useTheme } from 'next-themes'
import { themeTokens } from './callTheme'
import { DeviceSettings } from './DeviceSettings'

const FONT = '-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif'

// Единица-множитель для флюид-масштаба тулбара: плавно растёт с шириной вьюпорта.
// 1px до ~1440px (пол), до 1.5px к ~2200px+ (потолок). Все размеры = calc(N * var(--u)).
const U = 'clamp(1px, 0.069vw, 1.5px)'
const u = (n: number) => `calc(${n} * var(--u))`

// Иконки 15×15, stroke=currentColor, strokeWidth 1.8.
export const ICONS = {
  linkChain: (<><path d="M9 15l6-6" /><path d="M8 11L6.5 12.5a3.5 3.5 0 0 0 5 5L13 16" /><path d="M16 13l1.5-1.5a3.5 3.5 0 0 0-5-5L11 8" /></>),
  check: <path d="M20 6L9 17l-5-5" />,
  micOn: (<><path d="M12 2a3 3 0 0 0-3 3v6a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z" /><path d="M19 10v1a7 7 0 0 1-14 0v-1" /><path d="M12 18v4" /><path d="M8 22h8" /></>),
  micOff: (<><path d="M9 9v3a3 3 0 0 0 4.6 2.5" /><path d="M15 9.34V5a3 3 0 0 0-5.94-.6" /><path d="M19 10v1a7 7 0 0 1-.32 2.1" /><path d="M5 10v1a7 7 0 0 0 11.3 5.5" /><path d="M12 18v4" /><path d="M8 22h8" /><path d="M2 2l20 20" /></>),
  camOn: (<><path d="M15 8l5-3v14l-5-3" /><rect x="2" y="6" width="13" height="12" rx="2" /></>),
  camOff: (<><path d="M2 2l20 20" /><path d="M15 8l5-3v14l-2.2-1.32" /><path d="M2 8v10a2 2 0 0 0 2 2h9.5" /><path d="M2 6.5A2 2 0 0 1 4 5h9a2 2 0 0 1 2 2v2.5" /></>),
  share: (<><rect x="3" y="3" width="18" height="18" rx="3" /><path d="M12 16V8" /><path d="M9 11l3-3 3 3" /></>),
  board: (<><path d="M12 4H6a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-6" /><path d="M18.4 3.6a2.1 2.1 0 1 1 3 3L12 15.5l-4 1 1-4Z" /></>),
  chat: <path d="M21 11.5a8.5 8.5 0 0 1-8.5 8.5 8.4 8.4 0 0 1-3.8-.9L3 21l1.9-5.7a8.4 8.4 0 0 1-.9-3.8 8.5 8.5 0 0 1 8.5-8.5h.5a8.4 8.4 0 0 1 8 8v.5Z" />,
  homework: (<><path d="M4 4h9a3 3 0 0 1 3 3v13a2.5 2.5 0 0 0-2.5-2.5H4Z" /><path d="M20 4h-3a3 3 0 0 0-3 3v13a2.5 2.5 0 0 1 2.5-2.5H20Z" /></>),
  more: (<><circle cx="5" cy="12" r="1.6" /><circle cx="12" cy="12" r="1.6" /><circle cx="19" cy="12" r="1.6" /></>),
  leave: (<><path d="M9 21H6a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3" /><path d="M16 17l5-5-5-5" /><path d="M21 12H9" /></>),
}

function Icon({ children }: { children: ReactNode }) {
  return (
    <svg style={{ width: u(15), height: u(15) }} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      {children}
    </svg>
  )
}

interface Props {
  role: 'tutor' | 'guest'
  inviteUrl?: string
  boardActive: boolean
  chatActive: boolean
  chatUnread: number
  /** Домашка есть только у курсового урока. */
  showHomework?: boolean
  onToggleBoard: () => void
  onToggleChat: () => void
  onHomework: () => void
  onLeave: () => void
}

export function CallToolbar({
  role, inviteUrl, boardActive, chatActive, chatUnread, showHomework = true,
  onToggleBoard, onToggleChat, onHomework, onLeave,
}: Props) {
  const { resolvedTheme } = useTheme()
  const c = themeTokens(resolvedTheme === 'dark' ? 'dark' : 'light')
  const { localParticipant, isMicrophoneEnabled, isCameraEnabled, isScreenShareEnabled } =
    useLocalParticipant()

  const [copied, setCopied] = useState(false)
  const [moreOpen, setMoreOpen] = useState(false)
  const [confirmLeave, setConfirmLeave] = useState(false)
  const copyTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => () => { if (copyTimer.current) clearTimeout(copyTimer.current) }, [])

  // Закрытие меню «Ещё» по клику вне него (кнопка + попап помечены data-call-more).
  useEffect(() => {
    if (!moreOpen) return
    function onDown(e: PointerEvent) {
      if ((e.target as HTMLElement).closest('[data-call-more]')) return
      setMoreOpen(false)
    }
    document.addEventListener('pointerdown', onDown, true)
    return () => document.removeEventListener('pointerdown', onDown, true)
  }, [moreOpen])

  function copyLink() {
    if (!inviteUrl) return
    try { navigator.clipboard.writeText(inviteUrl) } catch {}
    setCopied(true)
    if (copyTimer.current) clearTimeout(copyTimer.current)
    copyTimer.current = setTimeout(() => setCopied(false), 2200)
  }

  const btnBase = (opts: { active?: boolean; danger?: boolean; copied?: boolean }): React.CSSProperties => {
    let bg = 'transparent'
    let color = c.text
    if (opts.danger) { bg = c.destructiveBg; color = c.destructive }
    else if (opts.copied) { bg = c.successBg; color = c.success }
    else if (opts.active) { bg = c.accentBg; color = c.accent }
    return {
      position: 'relative', width: u(32), height: u(32), minWidth: u(32),
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      padding: 0, border: 'none', borderRadius: u(8), background: bg, color,
      cursor: 'pointer', flexShrink: 0, transition: 'background .15s,color .15s',
    }
  }

  return (
    <div style={{ display: 'contents', '--u': U } as React.CSSProperties}>
      {/* Тост копирования */}
      {copied && (
        <div style={{
          position: 'absolute', top: 16, left: '50%', transform: 'translateX(-50%)',
          background: 'rgba(20,20,21,0.9)', color: '#fff', padding: `${u(8)} ${u(14)}`,
          borderRadius: u(10), display: 'flex', alignItems: 'center', gap: u(8),
          fontSize: u(13), fontWeight: 500, zIndex: 60, fontFamily: FONT,
        }}>
          <svg style={{ width: u(15), height: u(15) }} viewBox="0 0 24 24" fill="none" stroke="#4F9768" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round">
            <path d="M20 6L9 17l-5-5" />
          </svg>
          Ссылка на звонок скопирована
        </div>
      )}

      {/* Меню «Ещё» (вверх) */}
      {moreOpen && (
        <div data-call-more style={{
          position: 'absolute', right: 22, bottom: u(72), background: c.panel,
          border: `1px solid ${c.border}`, borderRadius: 12, padding: 6,
          display: 'flex', flexDirection: 'column', minWidth: 240,
          boxShadow: '0 16px 34px rgba(0,0,0,0.3)', zIndex: 20, fontFamily: FONT,
        }}>
          <DeviceSettings theme={c} />
        </div>
      )}

      {/* Модалка выхода */}
      {confirmLeave && (
        <div style={{
          position: 'absolute', inset: 0, background: 'rgba(0,0,0,0.5)',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          zIndex: 70, fontFamily: FONT,
        }}>
          <div style={{ width: 320, background: c.panel, borderRadius: 14, padding: 20, boxShadow: '0 20px 50px rgba(0,0,0,0.35)' }}>
            <div style={{ fontSize: 16, fontWeight: 700, color: c.text, marginBottom: 6 }}>Покинуть звонок?</div>
            <div style={{ fontSize: 13, color: c.muted, marginBottom: 18, lineHeight: 1.4 }}>Вы сможете вернуться по той же ссылке.</div>
            <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end' }}>
              <button onClick={() => setConfirmLeave(false)} style={{ border: `1px solid ${c.border}`, background: 'transparent', color: c.text, borderRadius: 9, padding: '8px 16px', fontSize: 13, fontWeight: 600, cursor: 'pointer', fontFamily: FONT }}>Отмена</button>
              <button onClick={onLeave} style={{ border: 'none', background: c.destructive, color: '#fff', borderRadius: 9, padding: '8px 16px', fontSize: 13, fontWeight: 600, cursor: 'pointer', fontFamily: FONT }}>Покинуть</button>
            </div>
          </div>
        </div>
      )}

      {/* Сам тулбар */}
      <div style={{
        position: 'absolute', left: '50%', bottom: u(14), transform: 'translateX(-50%)',
        display: 'flex', alignItems: 'center', gap: u(2), padding: `${u(5)} ${u(6)}`,
        borderRadius: u(10), background: c.panel, border: `1px solid ${c.border}`,
        boxShadow: '0 6px 18px rgba(0,0,0,0.22)', zIndex: 10, fontFamily: FONT,
      }}>
        {role === 'tutor' && inviteUrl && (
          <button onClick={copyLink} title="Скопировать ссылку" style={btnBase({ copied })}>
            <Icon>{copied ? ICONS.check : ICONS.linkChain}</Icon>
          </button>
        )}
        <button
          onClick={() => localParticipant.setMicrophoneEnabled(!isMicrophoneEnabled)}
          title="Микрофон" style={btnBase({})}
        >
          <Icon>{isMicrophoneEnabled ? ICONS.micOn : ICONS.micOff}</Icon>
        </button>
        <button
          onClick={() => localParticipant.setCameraEnabled(!isCameraEnabled)}
          title="Камера" style={btnBase({})}
        >
          <Icon>{isCameraEnabled ? ICONS.camOn : ICONS.camOff}</Icon>
        </button>
        <button
          onClick={() => localParticipant.setScreenShareEnabled(!isScreenShareEnabled)}
          title="Демонстрация экрана" style={btnBase({ active: isScreenShareEnabled })}
        >
          <Icon>{ICONS.share}</Icon>
        </button>
        {role === 'tutor' && (
          <button onClick={() => { setMoreOpen(false); onToggleBoard() }} title="Доска" style={btnBase({ active: boardActive })}>
            <Icon>{ICONS.board}</Icon>
          </button>
        )}
        {showHomework && (
          <button onClick={() => { setMoreOpen(false); onHomework() }} title="Домашнее задание" style={btnBase({})}>
            <Icon>{ICONS.homework}</Icon>
          </button>
        )}
        <button onClick={() => { setMoreOpen(false); onToggleChat() }} title="Чат" style={btnBase({ active: chatActive })}>
          <Icon>{ICONS.chat}</Icon>
          {chatUnread > 0 && !chatActive && (
            <span style={{
              position: 'absolute', top: u(-2), right: u(-2), minWidth: u(15), height: u(15),
              padding: `0 ${u(4)}`, borderRadius: u(8), background: c.destructive, color: '#fff',
              fontSize: u(9), fontWeight: 700, display: 'flex', alignItems: 'center', justifyContent: 'center',
            }}>{chatUnread}</span>
          )}
        </button>
        <button data-call-more onClick={() => setMoreOpen((v) => !v)} title="Ещё" style={btnBase({ active: moreOpen })}>
          <Icon>{ICONS.more}</Icon>
        </button>
        <button onClick={() => setConfirmLeave(true)} title="Покинуть звонок" style={btnBase({ danger: true })}>
          <Icon>{ICONS.leave}</Icon>
        </button>
      </div>
    </div>
  )
}
