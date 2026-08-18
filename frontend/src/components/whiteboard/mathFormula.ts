// Формула на доске — это image-элемент, у которого источник истины лежит в
// customData, а картинка каждый раз рендерится клиентом заново. Здесь — только
// чистые функции вокруг этого контракта, без React и без MathJax.

/** Версия формата: сменим схему — сможем отличить старые элементы. */
export type FormulaData = { latex: string; v: '1' }

export function formulaCustomData(latex: string): { formula: FormulaData } {
  return { formula: { latex, v: '1' } }
}

export function readFormula(
  el: { customData?: Record<string, unknown> } | null | undefined
): FormulaData | null {
  const raw = el?.customData?.formula as Partial<FormulaData> | undefined
  if (!raw || typeof raw.latex !== 'string') return null
  return { latex: raw.latex, v: '1' }
}

// FNV-1a, два прохода с разными смещениями → 64 бита. Хватает: коллизия
// означала бы показ чужой картинки, при сотнях формул на доске вероятность
// ничтожна, а crypto.subtle асинхронный и тут только мешал бы.
function fnv1a(input: string, seed: number): number {
  let h = seed
  for (let i = 0; i < input.length; i++) {
    h ^= input.charCodeAt(i)
    h = Math.imul(h, 0x01000193) >>> 0
  }
  return h >>> 0
}

/**
 * Ключ картинки. Цвет и кегль входят в хеш, потому что запечены в SVG:
 * Excalidraw пропускает addFiles для уже известного fileId, и без них смена
 * цвета формулы не перерисовала бы её.
 */
export function fileIdForLatex(latex: string, color: string, fontSize: number): string {
  const key = `${latex} ${color} ${fontSize}`
  const hi = fnv1a(key, 0x811c9dc5).toString(16).padStart(8, '0')
  const lo = fnv1a(key, 0x01000193).toString(16).padStart(8, '0')
  return `math-${hi}${lo}`
}

/**
 * Стоит ли прогонять выделенный текст через convertAsciiMathToLatex.
 *
 * Конвертер хорош на математике, но всегда съедает пробелы: «Задача 5»
 * становится «Задача5», то есть произведением курсивных переменных. Поэтому
 * кириллицу и фразы из слов не трогаем — открываем пустое поле.
 */
export function looksLikeMath(text: string): boolean {
  const s = text.trim()
  if (!s) return false
  if (/[Ѐ-ӿ]/.test(s)) return false
  if (/[A-Za-z]{2,}\s+[A-Za-z]{2,}/.test(s)) return false
  return /[0-9+\-*/^_=()]|sqrt|frac|pi|alpha|beta/.test(s)
}
