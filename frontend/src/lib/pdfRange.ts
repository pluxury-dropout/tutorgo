// Чистые хелперы вставки PDF. Держатся отдельно от pdf.ts (без импорта pdfjs),
// чтобы node:test мог их гонять.

// Ширина «полки» страниц: дальше раскладка переносится на новую строку, иначе
// 50-страничный документ уезжает в бесконечную ленту вправо.
export const PAGES_PER_ROW = 7

// Раскладывает страницы встык слева направо, по PAGES_PER_ROW в строке.
// Высота строки = самая высокая страница в ней: у PDF со смешанной ориентацией
// альбомная страница не должна наезжать на следующую строку.
export function layoutPages(
  sizes: readonly { w: number; h: number }[],
  origin: { x: number; y: number },
  perRow: number = PAGES_PER_ROW
): { x: number; y: number; w: number; h: number }[] {
  const placed: { x: number; y: number; w: number; h: number }[] = []
  let y = origin.y
  for (let i = 0; i < sizes.length; i += perRow) {
    const row = sizes.slice(i, i + perRow)
    let x = origin.x
    for (const s of row) {
      placed.push({ x, y, w: s.w, h: s.h })
      x += s.w
    }
    y += Math.max(...row.map((s) => s.h))
  }
  return placed
}

// Парсит пользовательский ввод диапазона страниц ("5", "5-8") в нормализованный
// [from, to]. Любой невалидный ввод трактуется как «весь документ».
export function parseRange(input: string, numPages: number): [number, number] {
  const clamp = (n: number) => Math.min(Math.max(n, 1), numPages)

  const trimmed = input.trim()
  if (trimmed === '') return [1, numPages]

  const parts = trimmed.split('-').map((p) => p.trim())
  const nums = parts.map((p) => Number(p))
  if (nums.some((n) => !Number.isInteger(n))) return [1, numPages]

  let from = clamp(nums[0])
  let to = nums.length > 1 ? clamp(nums[1]) : from
  if (from > to) [from, to] = [to, from]
  return [from, to]
}
