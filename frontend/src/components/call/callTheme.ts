// Токены и раскладка окна звонка. Чистый модуль (без React/LiveKit) —
// тестируется через node:test.

// Сцена звонка — «кинозал»: тёмная всегда, независимо от темы приложения.
// Поэтому здесь хардкод, а не var(--*).
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

// Ссылки на общие токены globals.css, а не своя копия палитры: тему переключает
// сам CSS по классу .dark, поэтому объект один и от resolvedTheme не зависит.
// Полупрозрачные фоны — через color-mix от того же токена, чтобы оттенок не
// разъезжался с базовым цветом при правке палитры.
export const CALL_THEME: CallTheme = {
  panel: 'var(--card)',
  border: 'var(--border)',
  borderSoft: 'color-mix(in srgb, var(--foreground) 10%, transparent)',
  text: 'var(--foreground)',
  muted: 'var(--muted-foreground)',
  hover: 'var(--muted)',
  accent: 'var(--primary)',
  accentBg: 'var(--primary-light)',
  destructive: 'var(--destructive)',
  destructiveBg: 'color-mix(in srgb, var(--destructive) 15%, transparent)',
  success: 'var(--success)',
  successBg: 'color-mix(in srgb, var(--success) 18%, transparent)',
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
