// Чистая арифметика месячной сетки: живёт отдельно от вёрстки, чтобы её можно
// было прогнать через node --test (JSX ему не по зубам).

export function isSameLocalDay(a: Date, b: Date): boolean {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  )
}

/** Ключ дня по локальному времени — для группировки уроков по клеткам. */
export function dayKey(d: Date): string {
  return `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`
}

/** Клетки месяца: null-заглушки до первого числа, затем сами дни. */
export function buildGrid(year: number, month: number): (Date | null)[] {
  const first = new Date(year, month, 1)
  // getDay(): 0=Вс…6=Сб → приводим к ISO, где неделя начинается с понедельника.
  const isoDay = first.getDay() === 0 ? 7 : first.getDay()
  const daysInMonth = new Date(year, month + 1, 0).getDate()

  const grid: (Date | null)[] = Array(isoDay - 1).fill(null)
  for (let d = 1; d <= daysInMonth; d++) grid.push(new Date(year, month, d))
  return grid
}

/**
 * Тот же день в соседнем месяце, в полночь. День обрезается по длине месяца:
 * иначе 31 марта − 1 месяц дало бы 31 февраля, а Date молча перекинул бы это
 * на 2–3 марта.
 */
export function shiftMonth(base: Date, delta: number): Date {
  const target = new Date(base.getFullYear(), base.getMonth() + delta, 1)
  const daysInMonth = new Date(target.getFullYear(), target.getMonth() + 1, 0).getDate()
  target.setDate(Math.min(base.getDate(), daysInMonth))
  return target
}
