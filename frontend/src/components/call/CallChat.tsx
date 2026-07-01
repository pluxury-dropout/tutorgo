// frontend/src/components/call/CallChat.tsx
'use client'

import { useEffect, useRef, useState } from 'react'
import type { CallTheme } from './callTheme'
import type { ChatMessage } from './callChat'

interface Props {
  theme: CallTheme
  messages: ChatMessage[]
  onSend: (text: string) => void
  onClose: () => void
}

export function CallChat({ theme, messages, onSend, onClose }: Props) {
  const [draft, setDraft] = useState('')
  const listRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (listRef.current) listRef.current.scrollTop = listRef.current.scrollHeight
  }, [messages])

  function submit() {
    if (!draft.trim()) return
    onSend(draft)
    setDraft('')
  }

  return (
    <div
      style={{
        position: 'absolute', top: 0, right: 0, bottom: 0, width: 280,
        background: theme.panel, borderLeft: `1px solid ${theme.border}`,
        display: 'flex', flexDirection: 'column', zIndex: 12, color: theme.text,
        fontFamily: '-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif',
      }}
    >
      <div
        style={{
          display: 'flex', alignItems: 'center', justifyContent: 'space-between',
          padding: '13px 14px', borderBottom: `1px solid ${theme.border}`,
        }}
      >
        <span style={{ fontWeight: 600, fontSize: 14 }}>Чат</span>
        <button
          onClick={onClose}
          aria-label="Закрыть чат"
          style={{
            width: 26, height: 26, borderRadius: 7, border: 'none',
            background: 'transparent', color: theme.muted, cursor: 'pointer',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M18 6L6 18" /><path d="M6 6l12 12" />
          </svg>
        </button>
      </div>

      <div ref={listRef} style={{ flex: 1, overflowY: 'auto', padding: '12px 14px', display: 'flex', flexDirection: 'column', gap: 10 }}>
        {messages.map((m, i) => (
          <div key={i} style={{ alignSelf: m.mine ? 'flex-end' : 'flex-start', maxWidth: '85%' }}>
            <div style={{ fontSize: 10.5, color: theme.muted, marginBottom: 2, textAlign: m.mine ? 'right' : 'left' }}>
              {m.from}
            </div>
            <div style={{ fontSize: 13, lineHeight: 1.4, padding: '7px 10px', borderRadius: 10, background: m.mine ? theme.accentBg : theme.hover, color: theme.text }}>
              {m.text}
            </div>
          </div>
        ))}
      </div>

      <div style={{ display: 'flex', gap: 8, padding: '11px 12px', borderTop: `1px solid ${theme.border}` }}>
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); submit() } }}
          placeholder="Написать сообщение…"
          style={{
            flex: 1, border: `1px solid ${theme.border}`,
            background: theme.panel === '#FFFFFF' ? '#F8F8F9' : '#1a1a1a',
            color: theme.text, borderRadius: 8, padding: '8px 10px', fontSize: 13,
            fontFamily: 'inherit', outline: 'none',
          }}
        />
        <button
          onClick={submit}
          aria-label="Отправить"
          style={{
            width: 34, height: 34, borderRadius: 8, border: 'none',
            background: theme.accent, color: '#fff', cursor: 'pointer', flexShrink: 0,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M22 2L11 13" /><path d="M22 2l-7 20-4-9-9-4 20-7Z" />
          </svg>
        </button>
      </div>
    </div>
  )
}
