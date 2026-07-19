import { test } from 'node:test'
import assert from 'node:assert/strict'
import { lerpCamera, camerasClose } from './viewportInterp.ts'

test('lerpCamera на t=0.5 берёт середину по всем осям', () => {
  const mid = lerpCamera(
    { scrollX: 0, scrollY: 0, zoom: 1 },
    { scrollX: 10, scrollY: 20, zoom: 2 },
    0.5
  )
  assert.equal(mid.scrollX, 5)
  assert.equal(mid.scrollY, 10)
  assert.equal(mid.zoom, 1.5)
})

test('lerpCamera клампит t за пределами [0,1]', () => {
  const to = { scrollX: 10, scrollY: 10, zoom: 2 }
  assert.deepEqual(lerpCamera({ scrollX: 0, scrollY: 0, zoom: 1 }, to, 5), to)
  assert.deepEqual(
    lerpCamera({ scrollX: 0, scrollY: 0, zoom: 1 }, to, -1),
    { scrollX: 0, scrollY: 0, zoom: 1 }
  )
})

test('camerasClose: почти совпали — true, далеко — false', () => {
  const base = { scrollX: 0, scrollY: 0, zoom: 1 }
  assert.equal(camerasClose(base, { scrollX: 0.1, scrollY: 0, zoom: 1 }), true)
  assert.equal(camerasClose(base, { scrollX: 5, scrollY: 0, zoom: 1 }), false)
})
