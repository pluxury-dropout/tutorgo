// frontend/src/lib/api/subscription.ts
import { api } from './client'

export type SubState = 'active' | 'grace' | 'blocked'
export type Plan = 'monthly' | 'yearly'

export interface Subscription {
  state: SubState
  plan: Plan | null
  period_end: string | null
  prices: { monthly: number; yearly: number; currency: string }
}

export const subscriptionApi = {
  get: () => api.get<Subscription>('/subscription').then((r) => r.data),
  // checkout возвращает { checkout_url } — заглушку отбрасываем (провайдера ещё нет).
  checkout: (plan: Plan) =>
    api.post('/subscription/checkout', { plan }).then(() => undefined),
  confirm: (plan: Plan) =>
    api.post('/subscription/confirm', { plan }).then(() => undefined),
}
