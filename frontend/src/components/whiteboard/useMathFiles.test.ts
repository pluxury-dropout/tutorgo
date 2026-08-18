import { test } from 'node:test'
import assert from 'node:assert/strict'
import { formulasNeedingRender } from './useMathFiles.ts'
import { formulaCustomData } from './mathFormula.ts'

const el = (over: Record<string, unknown>) => ({
  type: 'image',
  fileId: 'math-1',
  isDeleted: false,
  customData: formulaCustomData('x^2'),
  ...over,
})

test('возвращает формулы, которых нет ни в файлах, ни среди провалившихся', () => {
  const out = formulasNeedingRender([el({})] as never, new Set(), new Set())
  assert.deepEqual(out, [{ fileId: 'math-1', latex: 'x^2' }])
})

test('молчит, когда файл уже есть в карте файлов — иначе addFiles зациклит onChange', () => {
  const out = formulasNeedingRender([el({})] as never, new Set(['math-1']), new Set())
  assert.deepEqual(out, [])
})

test('молчит для формул, чей рендер уже провалился — их не будет в getFiles() никогда', () => {
  const out = formulasNeedingRender([el({})] as never, new Set(), new Set(['math-1']))
  assert.deepEqual(out, [])
})

test('игнорирует не-формулы и удалённые элементы', () => {
  const items = [
    el({ customData: undefined, fileId: 'photo-1' }),
    el({ isDeleted: true, fileId: 'math-2' }),
    { type: 'text', text: 'x^2', customData: formulaCustomData('x^2') },
  ]
  assert.deepEqual(formulasNeedingRender(items as never, new Set(), new Set()), [])
})

test('дедуплицирует одинаковые формулы в одном проходе', () => {
  const items = [el({}), el({ id: 'b' })]
  assert.equal(formulasNeedingRender(items as never, new Set(), new Set()).length, 1)
})
