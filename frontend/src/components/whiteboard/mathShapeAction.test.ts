// Кнопка «В формулу» встраивается порталом в чужую панель свойств: публичного
// API для этого у Excalidraw нет. Апгрейд пакета, переименовавший класс,
// уронил бы кнопку молча — этот тест падает вместо неё.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readdir, readFile } from 'node:fs/promises'

const DIST = 'node_modules/@excalidraw/excalidraw/dist/prod'

test('панель свойств Excalidraw всё ещё зовётся .panelColumn', async () => {
  const files = (await readdir(DIST)).filter((f) => f.endsWith('.js'))
  const sources = await Promise.all(files.map((f) => readFile(`${DIST}/${f}`, 'utf8')))
  assert.ok(
    sources.some((s) => s.includes('panelColumn')),
    'класс panelColumn исчез — сверь MathShapeAction.tsx с новой вёрсткой панели'
  )
})
