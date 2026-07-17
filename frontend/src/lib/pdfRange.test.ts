import { test } from 'node:test'
import assert from 'node:assert/strict'
import { parseRange, layoutPages } from './pdfRange.ts'

test('диапазон "5-8"', () => {
  assert.deepEqual(parseRange('5-8', 100), [5, 8])
})

test('одна страница "5"', () => {
  assert.deepEqual(parseRange('5', 100), [5, 5])
})

test('перевёрнутый "8-5" меняет местами', () => {
  assert.deepEqual(parseRange('8-5', 100), [5, 8])
})

test('ноль клампится к 1', () => {
  assert.deepEqual(parseRange('0', 100), [1, 1])
})

test('верх клампится к numPages', () => {
  assert.deepEqual(parseRange('3-999', 100), [3, 100])
})

test('пустая строка = весь документ', () => {
  assert.deepEqual(parseRange('', 100), [1, 100])
})

test('мусор = весь документ', () => {
  assert.deepEqual(parseRange('abc', 100), [1, 100])
})

test('пробелы игнорируются', () => {
  assert.deepEqual(parseRange(' 5 - 8 ', 100), [5, 8])
})

test('layoutPages: 8-я страница переносится на вторую строку', () => {
  const sizes = Array.from({ length: 9 }, () => ({ w: 100, h: 140 }))
  const placed = layoutPages(sizes, { x: 0, y: 0 })
  // первая строка — 7 страниц встык
  assert.deepEqual(
    placed.slice(0, 7).map((p) => p.x),
    [0, 100, 200, 300, 400, 500, 600]
  )
  assert.ok(placed.slice(0, 7).every((p) => p.y === 0))
  // восьмая начинает вторую строку с начала
  assert.deepEqual(placed[7], { x: 0, y: 140, w: 100, h: 140 })
  assert.deepEqual(placed[8], { x: 100, y: 140, w: 100, h: 140 })
})

test('layoutPages: высота строки — по самой высокой странице', () => {
  // альбомная страница в первой строке не должна дать наезд на вторую
  const sizes = [{ w: 100, h: 140 }, { w: 200, h: 80 }, { w: 100, h: 140 }]
  const placed = layoutPages(sizes, { x: 0, y: 0 }, 2)
  assert.equal(placed[2].y, 140)
})

test('layoutPages: раскладка стартует из точки drop, пустой вход — пусто', () => {
  assert.deepEqual(layoutPages([], { x: 5, y: 7 }), [])
  assert.deepEqual(layoutPages([{ w: 10, h: 20 }], { x: 5, y: 7 }), [
    { x: 5, y: 7, w: 10, h: 20 },
  ])
})
