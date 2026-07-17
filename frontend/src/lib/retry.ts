// Повтор транзиентных сбоев сети. Ошибки axios-клиента нормализованы в
// { message, status } (см. lib/api/client.ts), status = 0 при обрыве связи.

// Что имеет смысл повторять: сеть моргнула (0), троттлинг (429), сервер прилёг
// (5xx). Остальные 4xx детерминированы — 413 со второго раза не станет меньше,
// 403 не станет разрешением, повтор только тянет время.
export function isTransient(status: number): boolean {
  return status === 0 || status === 429 || status >= 500
}

export interface RetryOpts {
  attempts?: number
  /** База экспоненты; вынесена ради тестов — им незачем ждать по-настоящему. */
  delayMs?: number
  sleep?: (ms: number) => Promise<void>
}

// Экспоненциальный backoff со случайным разбросом: без джиттера пул из N
// параллельных заливок, словивший общий 5xx, синхронно постучится снова тем же
// залпом. Возвращает результат первой успешной попытки, иначе бросает последнюю
// ошибку — вызывающий не отличает «получилось сразу» от «получилось с третьей».
export async function withRetry<T>(
  fn: () => Promise<T>,
  { attempts = 3, delayMs = 500, sleep = defaultSleep }: RetryOpts = {}
): Promise<T> {
  for (let attempt = 0; ; attempt++) {
    try {
      return await fn()
    } catch (e) {
      const status = (e as { status?: number })?.status ?? 0
      if (attempt >= attempts || !isTransient(status)) throw e
      await sleep(2 ** attempt * delayMs * (0.5 + Math.random()))
    }
  }
}

const defaultSleep = (ms: number) =>
  new Promise<void>((r) => setTimeout(r, ms))
