// frontend/src/components/call/CallToolbar.tsx
'use client'

import { useEffect, useRef, useState } from 'react'
import { useLocalParticipant } from '@livekit/components-react'
import {
  BookOpen, Check, Link, LogOut, MessageCircle, Mic, MicOff, MoreHorizontal,
  ScreenShare, SquarePen, Video, VideoOff,
} from 'lucide-react'
import { CALL_THEME as c } from './callTheme'
import { DeviceSettings } from './DeviceSettings'

const FONT = '-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif'

// Размеры взяты у тулбара Excalidraw (--lg-button-size 2.25rem, --lg-icon-size
// 1rem): две панели стоят на одном экране, и любой свой масштаб здесь читается
// как рассинхрон. Флюид-множитель по вьюпорту был именно этим — Excalidraw с
// шириной экрана не растёт, а тулбар звонка уезжал в полтора раза.
const BTN = 36
const ICON = 16

const icon = { size: ICON, strokeWidth: 1.75 } as const

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

  // Фон отдаём через --btn-bg: hover живёт в CSS (.lesson-btn), иначе inline
  // перебил бы его. Выход — единственная залитая кнопка, как в макете.
  const btnBase = (opts: { active?: boolean; danger?: boolean; copied?: boolean }): React.CSSProperties => {
    let bg = 'transparent'
    let color = c.muted
    if (opts.danger) { bg = c.destructive; color = '#fff' }
    else if (opts.copied) { bg = c.successBg; color = c.success }
    else if (opts.active) { bg = c.accentBg; color = c.text }
    return {
      position: 'relative',
      width: opts.danger ? 40 : BTN, height: BTN, minWidth: opts.danger ? 40 : BTN,
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      padding: 0, border: 'none', borderRadius: opts.danger ? 12 : 10,
      ['--btn-bg' as string]: bg, color,
      cursor: 'pointer', flexShrink: 0,
    }
  }

  return (
    <div style={{ display: 'contents' }}>
      {/* Тост копирования */}
      {copied && (
        <div style={{
          position: 'absolute', top: 16, left: '50%', transform: 'translateX(-50%)',
          background: 'rgba(20,20,21,0.9)', color: '#fff', padding: '8px 14px',
          borderRadius: 10, display: 'flex', alignItems: 'center', gap: 8,
          fontSize: 13, fontWeight: 500, zIndex: 60, fontFamily: FONT,
        }}>
          <Check size={ICON} color="#4F9768" strokeWidth={2.4} />
          Ссылка на звонок скопирована
        </div>
      )}

      {/* Меню «Ещё» (вверх) */}
      {moreOpen && (
        <div data-call-more style={{
          position: 'absolute', right: 22, bottom: 62, background: c.panel,
          border: `1px solid ${c.border}`, borderRadius: 14, padding: 6,
          display: 'flex', flexDirection: 'column', minWidth: 240,
          boxShadow: '0 12px 32px rgba(0,0,0,0.18)', zIndex: 20, fontFamily: FONT,
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
        position: 'absolute', left: '50%', bottom: 14, transform: 'translateX(-50%)',
        display: 'flex', alignItems: 'center', gap: 2, padding: '4px 6px',
        borderRadius: 14, background: c.panel, border: `1px solid ${c.border}`,
        boxShadow: '0 4px 16px rgba(0,0,0,0.10)', zIndex: 10, fontFamily: FONT,
      }}>
        {role === 'tutor' && inviteUrl && (
          <button className="lesson-btn" onClick={copyLink} title="Скопировать ссылку" style={btnBase({ copied })}>
            {copied ? <Check {...icon} /> : <Link {...icon} />}
          </button>
        )}
        {/* Выключенные микрофон и камера подсвечены — это состояние, а не выбор
            инструмента: молчащий человек должен видеть, что он молчит. */}
        <button
          className="lesson-btn"
          onClick={() => localParticipant.setMicrophoneEnabled(!isMicrophoneEnabled)}
          title="Микрофон" style={btnBase({ active: !isMicrophoneEnabled })}
        >
          {isMicrophoneEnabled ? <Mic {...icon} /> : <MicOff {...icon} />}
        </button>
        <button
          className="lesson-btn"
          onClick={() => localParticipant.setCameraEnabled(!isCameraEnabled)}
          title="Камера" style={btnBase({ active: !isCameraEnabled })}
        >
          {isCameraEnabled ? <Video {...icon} /> : <VideoOff {...icon} />}
        </button>
        <button
          className="lesson-btn"
          onClick={() => localParticipant.setScreenShareEnabled(!isScreenShareEnabled)}
          title="Демонстрация экрана" style={btnBase({ active: isScreenShareEnabled })}
        >
          <ScreenShare {...icon} />
        </button>
        {role === 'tutor' && (
          <button className="lesson-btn" onClick={() => { setMoreOpen(false); onToggleBoard() }} title="Доска" style={btnBase({ active: boardActive })}>
            <SquarePen {...icon} />
          </button>
        )}
        {showHomework && (
          <button className="lesson-btn" onClick={() => { setMoreOpen(false); onHomework() }} title="Домашнее задание" style={btnBase({})}>
            <BookOpen {...icon} />
          </button>
        )}
        <button className="lesson-btn" onClick={() => { setMoreOpen(false); onToggleChat() }} title="Чат" style={btnBase({ active: chatActive })}>
          <MessageCircle {...icon} />
          {chatUnread > 0 && !chatActive && (
            <span style={{
              position: 'absolute', top: -2, right: -2, minWidth: 15, height: 15,
              padding: '0 4px', borderRadius: 8, background: c.destructive, color: '#fff',
              fontSize: 9, fontWeight: 700, display: 'flex', alignItems: 'center', justifyContent: 'center',
            }}>{chatUnread}</span>
          )}
        </button>
        <button className="lesson-btn" data-call-more onClick={() => setMoreOpen((v) => !v)} title="Ещё" style={btnBase({ active: moreOpen })}>
          <MoreHorizontal {...icon} />
        </button>
        <div style={{ width: 1, height: 22, background: c.border, margin: '0 4px' }} />
        <button
          className="lesson-btn lesson-btn--danger"
          onClick={() => setConfirmLeave(true)}
          title="Покинуть звонок" style={btnBase({ danger: true })}
        >
          <LogOut {...icon} />
        </button>
      </div>
    </div>
  )
}
