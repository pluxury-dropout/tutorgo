import type { SubState } from './api/subscription'

export const PAYWALL_PATH = '/subscription'

export type GuardDecision =
  | { action: 'redirect' }
  | { action: 'render'; banner: boolean }

// Зеркалит серверный RequireActiveSubscription. blocked вне paywall → на paywall;
// на самом paywall всегда render (иначе цикл). grace → пускаем, но с баннером.
export function decideAccess(state: SubState, path: string): GuardDecision {
  if (state === 'blocked' && path !== PAYWALL_PATH) return { action: 'redirect' }
  return { action: 'render', banner: state === 'grace' }
}
