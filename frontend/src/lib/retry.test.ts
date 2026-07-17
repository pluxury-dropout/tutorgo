import { test } from 'node:test'
import assert from 'node:assert/strict'
import { withRetry, isTransient } from './retry.ts'

// Спим мгновенно и пишем задержки — тесту незачем ждать по-настоящему.
const fakeSleep = (log: number[]) => (ms: number) => {
  log.push(ms)
  return Promise.resolve()
}

const fail = (status: number) => Object.assign(new Error('нет'), { status })

test('isTransient: повторяем сеть, троттлинг и 5xx, но не 4xx', () => {
  assert.ok(isTransient(0)) // обрыв связи
  assert.ok(isTransient(429))
  assert.ok(isTransient(500))
  assert.ok(isTransient(503))
  assert.ok(!isTransient(400))
  assert.ok(!isTransient(413)) // файл больше не станет
  assert.ok(!isTransient(403))
})

test('withRetry: успех со второй попытки — результат возвращается', async () => {
  let calls = 0
  const r = await withRetry(
    () => {
      calls++
      return calls === 1 ? Promise.reject(fail(503)) : Promise.resolve('ок')
    },
    { sleep: fakeSleep([]) }
  )
  assert.equal(r, 'ок')
  assert.equal(calls, 2)
})

test('withRetry: 413 не повторяется — бросает сразу', async () => {
  let calls = 0
  await assert.rejects(
    withRetry(
      () => {
        calls++
        return Promise.reject(fail(413))
      },
      { sleep: fakeSleep([]) }
    ),
    /нет/
  )
  assert.equal(calls, 1)
})

test('withRetry: попытки кончились — бросает последнюю ошибку', async () => {
  let calls = 0
  await assert.rejects(
    withRetry(
      () => {
        calls++
        return Promise.reject(fail(500))
      },
      { attempts: 2, sleep: fakeSleep([]) }
    ),
    /нет/
  )
  assert.equal(calls, 3) // первая + 2 повтора
})

test('withRetry: пауза растёт экспоненциально', async () => {
  const delays: number[] = []
  await assert.rejects(
    withRetry(() => Promise.reject(fail(0)), {
      attempts: 3,
      delayMs: 100,
      sleep: fakeSleep(delays),
    })
  )
  assert.equal(delays.length, 3)
  // джиттер даёт разброс ×0.5…×1.5 вокруг 100 / 200 / 400
  const bounds = [100, 200, 400]
  delays.forEach((d, k) => {
    assert.ok(d >= bounds[k] * 0.5 && d <= bounds[k] * 1.5, `пауза ${k}: ${d}`)
  })
})
