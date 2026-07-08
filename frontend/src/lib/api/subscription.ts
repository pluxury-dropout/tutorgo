// frontend/src/lib/api/subscription.ts
import { api } from './client'

export type SubState = 'active' | 'grace' | 'blocked'
export type Plan = 'monthly' | 'yearly'

export interface Subscription {
  state: SubState
  plan: Plan | null
  period_end: string | null
  prices: { monthly: number; yearly: number; currency: string }
  autopay: boolean
  pending_plan: Plan | null
}

export const subscriptionApi = {
  get: () => api.get<Subscription>('/subscription').then((r) => r.data),
  // checkout → hosted-страница провайдера; редирект делает вызывающий.
  checkout: (plan: Plan) =>
    api.post<{ checkout_url: string }>('/subscription/checkout', { plan }).then((r) => r.data),
  cancel: () => api.post('/subscription/cancel').then(() => undefined),
  changePlan: (plan: Plan) =>
    api.post('/subscription/change-plan', { plan }).then(() => undefined),
}
