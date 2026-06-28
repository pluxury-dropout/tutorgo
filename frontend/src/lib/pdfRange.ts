// Парсит пользовательский ввод диапазона страниц ("5", "5-8") в нормализованный
// [from, to]. Любой невалидный ввод трактуется как «весь документ».
// Держится отдельно от pdf.ts (без импорта pdfjs), чтобы node:test мог его гонять.
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
