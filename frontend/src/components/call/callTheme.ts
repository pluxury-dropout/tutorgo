// Токены и раскладка окна звонка. Чистый модуль (без React/LiveKit) —
// тестируется через node:test.

export const STAGE_BG = '#101113'
export const TILE_BG = '#232427'
export const AVATAR = '#5b5d63'
export const GLYPH = '#3f4046'

export interface CallTheme {
  panel: string
  border: string
  borderSoft: string
  text: string
  muted: string
  hover: string
  accent: string
  accentBg: string
  destructive: string
  destructiveBg: string
  success: string
  successBg: string
}

const DARK: CallTheme = {
  panel: '#222222', border: '#2e2e2e', borderSoft: 'rgba(255,255,255,0.1)',
  text: '#E3E2E0', muted: '#979A9B', hover: '#3a3a3a',
  accent: '#6CA6E0', accentBg: 'rgba(108,166,224,0.16)',
  destructive: '#CD4945', destructiveBg: 'rgba(205,73,69,0.15)',
  success: '#4F9768', successBg: 'rgba(79,151,104,0.18)',
}

const LIGHT: CallTheme = {
  panel: '#FFFFFF', border: '#E1E1E4', borderSoft: 'rgba(27,28,31,0.1)',
  text: '#1B1C1F', muted: '#646670', hover: '#ECECEE',
  accent: '#1D4ED8', accentBg: 'rgba(29,78,216,0.08)',
  destructive: '#C92A2A', destructiveBg: 'rgba(201,42,42,0.08)',
  success: '#077A4E', successBg: 'rgba(7,122,78,0.1)',
}

export function themeTokens(resolved: 'dark' | 'light'): CallTheme {
  return resolved === 'dark' ? DARK : LIGHT
}

export interface StageLayout {
  mode: 'single' | 'grid'
  columns: number
}

// n<=1 → один центрированный tile; n===2 (кейс 1:1 урока) → 2 колонки;
// n>=3 → 3 колонки. Не хардкодим 3 — иначе 2 участника «проваливаются».
export function layoutForCount(n: number): StageLayout {
  if (n <= 1) return { mode: 'single', columns: 1 }
  if (n === 2) return { mode: 'grid', columns: 2 }
  return { mode: 'grid', columns: 3 }
}
