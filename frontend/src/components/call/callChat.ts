// Чистый codec чат-сообщений. Чат и доска делят один DataChannel, поэтому
// дискриминант `type` обязателен, чтобы обработчики не глотали чужие сообщения.

export interface ChatMessage {
  from: string
  text: string
  mine: boolean
}

interface ChatWire {
  type: 'chat'
  from: string
  text: string
}

export function encodeChat(from: string, text: string): Uint8Array {
  const wire: ChatWire = { type: 'chat', from, text }
  return new TextEncoder().encode(JSON.stringify(wire))
}

export function parseChatMessage(payload: Uint8Array): { from: string; text: string } | null {
  let msg: unknown
  try {
    msg = JSON.parse(new TextDecoder().decode(payload))
  } catch {
    return null
  }
  if (
    typeof msg === 'object' && msg !== null &&
    (msg as ChatWire).type === 'chat' &&
    typeof (msg as ChatWire).from === 'string' &&
    typeof (msg as ChatWire).text === 'string'
  ) {
    return { from: (msg as ChatWire).from, text: (msg as ChatWire).text }
  }
  return null
}
