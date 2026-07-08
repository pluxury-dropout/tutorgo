import type { SubState } from './api/subscription.ts'

export type PollResult = 'activated' | 'timeout' | 'wait'

// active выигрывает всегда; иначе ждём, пока есть попытки, потом таймаут.
export function pollDecision(state: SubState, attemptsLeft: number): PollResult {
  if (state === 'active') return 'activated'
  if (attemptsLeft <= 0) return 'timeout'
  return 'wait'
}
