import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  readFormula,
  formulaCustomData,
  fileIdForLatex,
  looksLikeMath,
} from './mathFormula.ts'

test('fileId детерминирован и меняется от каждого входа', () => {
  const a = fileIdForLatex('x^2', '#1e1e1e', 20)
  assert.equal(a, fileIdForLatex('x^2', '#1e1e1e', 20))
  // Цвет и кегль запечены в SVG: без них addFiles пропустил бы новый файл
  // («file data is not updated») и картинка не перерисовалась бы.
  assert.notEqual(a, fileIdForLatex('x^3', '#1e1e1e', 20))
  assert.notEqual(a, fileIdForLatex('x^2', '#e03131', 20))
  assert.notEqual(a, fileIdForLatex('x^2', '#1e1e1e', 28))
})

test('fileId — короткая hex-строка', () => {
  assert.match(fileIdForLatex('x^2', '#1e1e1e', 20), /^math-[0-9a-f]{16}$/)
})

test('readFormula достаёт формулу и отсеивает чужие элементы', () => {
  assert.deepEqual(readFormula({ customData: formulaCustomData('E=mc^2') }), {
    latex: 'E=mc^2',
    v: '1',
  })
  assert.equal(readFormula({ customData: { generationData: {} } }), null)
  assert.equal(readFormula({}), null)
  assert.equal(readFormula(null), null)
  // мусор из будущего формата не должен ронять чтение
  assert.equal(readFormula({ customData: { formula: { latex: 42 } } }), null)
})

test('гейт пропускает математику и отсекает текст', () => {
  assert.ok(looksLikeMath('x^2+1'))
  assert.ok(looksLikeMath('1/2'))
  assert.ok(looksLikeMath('E=mc^2'))
  assert.ok(looksLikeMath('sqrt(2)'))

  // convertAsciiMathToLatex уничтожает пробелы: «Задача 5» превратилась бы
  // в произведение шести курсивных переменных.
  assert.equal(looksLikeMath('Задача 5'), false)
  assert.equal(looksLikeMath('Hello world'), false)
  assert.equal(looksLikeMath('Найти скорость'), false)
  assert.equal(looksLikeMath(''), false)
})
