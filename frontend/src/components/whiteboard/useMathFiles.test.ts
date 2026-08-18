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

test('возвращает формулы, которых нет в кэше', () => {
  const out = formulasNeedingRender([el({})] as never, new Set())
  assert.deepEqual(out, [{ fileId: 'math-1', latex: 'x^2' }])
})

test('молчит, когда всё уже отрендерено — иначе addFiles зациклит onChange', () => {
  const out = formulasNeedingRender([el({})] as never, new Set(['math-1']))
  assert.deepEqual(out, [])
})

test('игнорирует не-формулы и удалённые элементы', () => {
  const items = [
    el({ customData: undefined, fileId: 'photo-1' }),
    el({ isDeleted: true, fileId: 'math-2' }),
    { type: 'text', text: 'x^2', customData: formulaCustomData('x^2') },
  ]
  assert.deepEqual(formulasNeedingRender(items as never, new Set()), [])
})

test('дедуплицирует одинаковые формулы в одном проходе', () => {
  const items = [el({}), el({ id: 'b' })]
  assert.equal(formulasNeedingRender(items as never, new Set()).length, 1)
})
