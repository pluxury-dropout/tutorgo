// Чистая арифметика дропа в недельной сетке телефона: живёт отдельно от вёрстки,
// чтобы её можно было прогнать через node --test (JSX ему не по зубам).
// Константы тут же — сетка и попадание в неё должны меняться заодно.

export const HOUR_PX   = 56
export const HOURS     = [8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21]
export const GRID_H    = HOURS.length * HOUR_PX
/** Колонка с подписями часов слева от дней. */
export const GUTTER_PX = 30
export const SNAP_MIN  = 30

function clamp(n: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, n))
}

/** Куда попадёт блок, отпущенный в позиции ghost поверх сетки grid.
 *  День — по центру блока (палец держит его где угодно), время — по верхней
 *  кромке, с шагом SNAP_MIN. Оба зажаты в границы сетки: бросок мимо неё
 *  кладёт блок в ближайший слот, а не улетает в другой день или в ночь. */
export function dropSlot(
  ghost: { left: number; top: number; width: number },
  grid:  { left: number; top: number; width: number },
  durationMinutes: number,
): { day: number; minutes: number } {
  const colW = (grid.width - GUTTER_PX) / 7
  const day  = clamp(Math.floor((ghost.left + ghost.width / 2 - grid.left - GUTTER_PX) / colW), 0, 6)

  const rawMin  = HOURS[0] * 60 + ((ghost.top - grid.top) / HOUR_PX) * 60
  const minutes = clamp(
    Math.round(rawMin / SNAP_MIN) * SNAP_MIN,
    HOURS[0] * 60,
    (HOURS[0] + HOURS.length) * 60 - durationMinutes,
  )

  return { day, minutes }
}
