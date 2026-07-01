export type UiTool =
  | 'select' | 'hand' | 'pen' | 'eraser' | 'text' | 'shape' | 'sticky' | 'image'

export const TOOL_IDS: UiTool[] = [
  'select', 'hand', 'pen', 'eraser', 'text', 'shape', 'sticky', 'image',
]

const EDITOR_TOOL: Record<UiTool, string | null> = {
  select: 'select', hand: 'hand', pen: 'draw', eraser: 'eraser',
  text: 'text', shape: 'geo', sticky: 'note', image: null,
}
export function toEditorTool(t: UiTool): string | null {
  return EDITOR_TOOL[t]
}

// hex мокапа → имя цвета tldraw (DefaultColorStyle)
export const COLORS = ['#26262a', '#e0564f', '#e6a43c', '#4f9d6e', '#4f7bd0', '#9168d6']
export const COLOR_MAP: Record<string, string> = {
  '#26262a': 'black', '#e0564f': 'red', '#e6a43c': 'orange',
  '#4f9d6e': 'green', '#4f7bd0': 'blue', '#9168d6': 'violet',
}

export const SIZE_KEYS = ['S', 'M', 'L', 'XL'] as const
export const SIZE_MAP: Record<string, string> = { S: 's', M: 'm', L: 'l', XL: 'xl' }
// диаметр точки-превью толщины (px), из мокапа
export const SIZE_DOT: Record<string, number> = { S: 4, M: 7, L: 11, XL: 15 }

export const FONT_DEFS = [
  { key: 'hand', label: 'Аа', family: "'Marck Script', cursive" },
  { key: 'sans', label: 'Аа', family: 'Inter, sans-serif' },
  { key: 'serif', label: 'Аа', family: 'Georgia, serif' },
]
export const FONT_MAP: Record<string, string> = { hand: 'draw', sans: 'sans', serif: 'serif' }

export function showColor(t: UiTool | null): boolean {
  return t === 'pen' || t === 'text' || t === 'shape' || t === 'sticky'
}
export function showThickness(t: UiTool | null): boolean {
  return t === 'pen' || t === 'eraser' || t === 'shape'
}
export function showFont(t: UiTool | null): boolean {
  return t === 'text'
}
export function hasSettings(t: UiTool | null): boolean {
  return !!t && (showColor(t) || showThickness(t) || showFont(t) || t === 'eraser')
}
