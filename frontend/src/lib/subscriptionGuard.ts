import type { SubState } from './api/subscription'

export const PAYWALL_PATH = '/subscription'

export type GuardDecision =
  | { action: 'redirect' }
  | { action: 'render'; banner: boolean }

// Зеркалит серверный RequireActiveSubscription. blocked вне платёжной воронки →
// на paywall; внутри воронки (/subscription и /subscription/success) всегда render
// (иначе цикл + success-страница не смогла бы дополлить активацию). grace → баннер.
function inPaymentFunnel(path: string): boolean {
  return path === PAYWALL_PATH || path.startsWith(PAYWALL_PATH + '/')
}

export function decideAccess(state: SubState, path: string): GuardDecision {
  if (state === 'blocked' && !inPaymentFunnel(path)) return { action: 'redirect' }
  return { action: 'render', banner: state === 'grace' }
}
