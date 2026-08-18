import { test } from 'node:test'
import assert from 'node:assert/strict'
import { latexToSvg, svgToDataUrl } from './latexToSvg.ts'

const QUADRATIC = 'x=\\frac{-b\\pm\\sqrt{b^2-4ac}}{2a}'

test('рендерит самодостаточный SVG с px-размерами и заданным цветом', async () => {
  const r = await latexToSvg(QUADRATIC, { fontSize: 20, color: '#1971c2' })

  assert.equal(r.error, undefined)
  assert.ok(r.svg.includes('xmlns="http://www.w3.org/2000/svg"'))
  // currentColor внутри <img> не резолвится — всё стало бы чёрным.
  assert.ok(!r.svg.includes('currentColor'))
  assert.ok(r.svg.includes('#1971c2'))
  // width/height обязаны быть в px: единица ex в standalone SVG резолвится
  // от системного шрифта, а не от нашего.
  assert.match(r.svg, /width="\d+(\.\d+)?"/)
  assert.ok(!/width="[\d.]+ex"/.test(r.svg))
  assert.ok(r.width > 150 && r.width < 220, `ширина ${r.width}`)
  assert.ok(r.height > 40 && r.height < 55, `высота ${r.height}`)
})

test('размер линеен по кеглю', async () => {
  const big = await latexToSvg(QUADRATIC, { fontSize: 20 })
  const small = await latexToSvg(QUADRATIC, { fontSize: 10 })
  assert.ok(Math.abs(small.width - big.width / 2) < 0.05)
  assert.ok(Math.abs(small.height - big.height / 2) < 0.05)
})

test('кириллица рендерится глифами, а не системным шрифтом', async () => {
  const r = await latexToSvg('\\text{Скорость } v=\\frac{s}{t}')
  assert.equal(r.error, undefined)
  // <text> означает, что MathJax сдался и зовёт шрифт зрителя.
  assert.ok(!r.svg.includes('<text'), 'SVG перестал быть самодостаточным')
})

test('битый LaTeX не кидает, а возвращает error', async () => {
  const r = await latexToSvg('\\frac{1}{')
  assert.equal(r.error, 'Missing close brace')
})

test('dataURL — base64 и переживает кириллицу', async () => {
  const r = await latexToSvg('\\text{Масса}')
  const url = svgToDataUrl(r.svg)
  assert.ok(url.startsWith('data:image/svg+xml;base64,'))
  const back = Buffer.from(url.split(',')[1], 'base64').toString('utf8')
  assert.ok(back.includes('svg'))
})
