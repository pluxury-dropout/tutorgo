export type Camera = { scrollX: number; scrollY: number; zoom: number }

// Линейная интерполяция камеры. t клампится в [0,1]: устойчиво к рывкам dt.
export function lerpCamera(from: Camera, to: Camera, t: number): Camera {
  const k = t < 0 ? 0 : t > 1 ? 1 : t
  return {
    scrollX: from.scrollX + (to.scrollX - from.scrollX) * k,
    scrollY: from.scrollY + (to.scrollY - from.scrollY) * k,
    zoom: from.zoom + (to.zoom - from.zoom) * k,
  }
}

// Камеры практически совпали — интерполяцию можно остановить до нового кадра.
export function camerasClose(
  a: Camera,
  b: Camera,
  posEps = 0.5,
  zoomEps = 0.001
): boolean {
  return (
    Math.abs(a.scrollX - b.scrollX) < posEps &&
    Math.abs(a.scrollY - b.scrollY) < posEps &&
    Math.abs(a.zoom - b.zoom) < zoomEps
  )
}
