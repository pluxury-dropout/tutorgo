import { test } from 'node:test'
import assert from 'node:assert/strict'
import { parseRange } from './pdfRange.ts'

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
