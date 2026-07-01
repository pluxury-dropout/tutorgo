'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import { useRoomContext } from '@livekit/components-react'
import { RoomEvent, type RemoteParticipant } from 'livekit-client'
import { encodeChat, parseChatMessage, type ChatMessage } from './callChat'

// Живёт в CallRoomInner: слушатель не должен отваливаться при закрытом чате.
export function useCallChat({ chatOpen }: { chatOpen: boolean }) {
  const room = useRoomContext()
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [unread, setUnread] = useState(0)

  // Актуальный chatOpen для обработчика без пересоздания подписки.
  const chatOpenRef = useRef(chatOpen)
  useEffect(() => {
    chatOpenRef.current = chatOpen
    if (chatOpen) setUnread(0)
  }, [chatOpen])

  useEffect(() => {
    function onData(payload: Uint8Array, participant?: RemoteParticipant) {
      const parsed = parseChatMessage(payload)
      if (!parsed) return // board-open/close и прочее — не наше
      const from = participant?.name || participant?.identity || parsed.from
      setMessages((prev) => [...prev, { from, text: parsed.text, mine: false }])
      if (!chatOpenRef.current) setUnread((u) => u + 1)
    }
    room.on(RoomEvent.DataReceived, onData)
    return () => { room.off(RoomEvent.DataReceived, onData) }
  }, [room])

  const send = useCallback((text: string) => {
    const trimmed = text.trim()
    if (!trimmed) return
    const from = room.localParticipant.name || room.localParticipant.identity
    room.localParticipant
      .publishData(encodeChat(from, trimmed), { reliable: true })
      .catch(() => {})
    setMessages((prev) => [...prev, { from: 'Вы', text: trimmed, mine: true }])
  }, [room])

  return { messages, send, unread }
}
