// Пакет не объявляет JSX-типов. Нам достаточно тега: сам элемент создаётся
// через new MathfieldElement() в эффекте, поэтому в JSX он не рендерится.
import type { MathfieldElement } from 'mathlive'

declare global {
  interface HTMLElementTagNameMap {
    'math-field': MathfieldElement
  }
}
