/** Сумма за уроки курса в целых тенге. Цена хранится пакетом «N уроков за X ₸»,
 *  цена урока — производная и бывает дробной (85 000 / 12 = 7 083,33…). Поэтому
 *  кратное пакету — ровно пакеты, остальное — уроки × цена урока с округлением
 *  до тенге (спека 2026-09-06, п. 4.4). */
export function amountFor(lessons: number, pricePerCycle: number, lessonsPerCycle: number): number {
  if (lessons <= 0) return 0
  const n = lessonsPerCycle > 0 ? lessonsPerCycle : 1
  if (lessons % n === 0) return (lessons / n) * pricePerCycle
  return Math.round((lessons * pricePerCycle) / n)
}
