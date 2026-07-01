import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  toEditorTool, COLOR_MAP, SIZE_MAP, FONT_MAP,
  showColor, showThickness, showFont, hasSettings, TOOL_IDS,
} from './boardTools.ts'

test('toEditorTool маппит инструменты', () => {
  assert.equal(toEditorTool('pen'), 'draw')
  assert.equal(toEditorTool('shape'), 'geo')
  assert.equal(toEditorTool('sticky'), 'note')
  assert.equal(toEditorTool('select'), 'select')
  assert.equal(toEditorTool('image'), null)
})

test('COLOR_MAP — hex мокапа в имена tldraw', () => {
  assert.equal(COLOR_MAP['#26262a'], 'black')
  assert.equal(COLOR_MAP['#e0564f'], 'red')
  assert.equal(COLOR_MAP['#4f7bd0'], 'blue')
})

test('SIZE_MAP и FONT_MAP', () => {
  assert.equal(SIZE_MAP['M'], 'm')
  assert.equal(SIZE_MAP['XL'], 'xl')
  assert.equal(FONT_MAP['hand'], 'draw')
  assert.equal(FONT_MAP['serif'], 'serif')
})

test('видимость секций поповера', () => {
  assert.equal(showColor('pen'), true)
  assert.equal(showColor('eraser'), false)
  assert.equal(showThickness('eraser'), true)
  assert.equal(showFont('text'), true)
  assert.equal(hasSettings('hand'), false)
  assert.equal(hasSettings('pen'), true)
})

test('TOOL_IDS — 8 инструментов в порядке мокапа', () => {
  assert.deepEqual(TOOL_IDS, ['select','hand','pen','eraser','text','shape','sticky','image'])
})
