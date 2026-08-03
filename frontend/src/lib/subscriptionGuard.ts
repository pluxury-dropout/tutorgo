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

// Статус не загрузился — чем это считать. Сервер ответил (403/500/…) → доверяем
// ответу и закрываемся (fail-closed). Связи не было (status 0 после ретраев) →
// null = «пока не знаем», спиннер вместо ложного paywall: иначе одно моргание
// мобильной сети выкидывает на оплату до перезагрузки страницы.
export function stateOnLoadError(status: number | undefined): SubState | null {
  return status === 0 ? null : 'blocked'
}
