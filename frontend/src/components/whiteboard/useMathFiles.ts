'use client'

import { useCallback, useRef, type RefObject } from 'react'
import type { ExcalidrawImperativeAPI, BinaryFileData, DataURL } from '@excalidraw/excalidraw/types'
import type { ExcalidrawElement, FileId } from '@excalidraw/excalidraw/element/types'
// Расширение обязательно: node --test резолвит ESM без бандлера и не
// достраивает .ts сам (см. src/lib/subscriptionPoll.ts — тот же приём).
import { readFormula } from './mathFormula.ts'

/**
 * Формулы сцены, для которых картинки ещё нет.
 *
 * Отдельная чистая функция, потому что это единственная логика, которую можно
 * проверить тестом: остальное — вызовы Excalidraw.
 */
export function formulasNeedingRender(
  elements: readonly ExcalidrawElement[],
  known: ReadonlySet<string>
): { fileId: string; latex: string }[] {
  const out: { fileId: string; latex: string }[] = []
  const seen = new Set(known)
  for (const el of elements) {
    if (el.type !== 'image' || el.isDeleted || !el.fileId) continue
    const formula = readFormula(el)
    if (!formula || seen.has(el.fileId)) continue
    seen.add(el.fileId)
    out.push({ fileId: el.fileId, latex: formula.latex })
  }
  return out
}

/**
 * Держит картинки формул в актуальном состоянии: LaTeX едет по сети, SVG
 * рендерится локально у каждого участника.
 *
 * Вызывается из onChange, то есть на КАЖДЫЙ кадр панорамирования — поэтому
 * ранний выход обязан быть дешёвым. И главное: api.addFiles() безусловно зовёт
 * scene.triggerUpdate() (даже когда ничего не добавил), а тот вызывает onChange
 * снова. Без множества уже отрендеренного это бесконечный цикл.
 */
export function useMathFiles(apiRef: RefObject<ExcalidrawImperativeAPI | null>) {
  const doneRef = useRef(new Set<string>())
  const inFlightRef = useRef(false)

  const renderMissing = useCallback(() => {
    const api = apiRef.current
    if (!api || inFlightRef.current) return

    const pending = formulasNeedingRender(api.getSceneElements(), doneRef.current)
    if (pending.length === 0) return

    inFlightRef.current = true
    void (async () => {
      try {
        // Модуль тяжёлый (~490 КБ gzip) — грузим при первой формуле на странице.
        const { latexToSvg, svgToDataUrl } = await import('@/lib/latexToSvg')
        const files: BinaryFileData[] = []

        for (const { fileId, latex } of pending) {
          const { svg, error } = await latexToSvg(latex, { color: colorOf(api, fileId) })
          // Битый SVG не отдаём: Excalidraw пометил бы элемент status:'error'
          // через newElementWith, а это version++ — порча уехала бы всем пирам.
          if (error || !svg) {
            doneRef.current.add(fileId)
            continue
          }
          doneRef.current.add(fileId)
          files.push({
            id: fileId as FileId,
            dataURL: svgToDataUrl(svg) as DataURL,
            mimeType: 'image/svg+xml',
            created: Date.now(),
          })
        }

        if (files.length > 0) apiRef.current?.addFiles(files)
      } catch (err) {
        console.warn('Формулу не удалось отрендерить', err)
      } finally {
        inFlightRef.current = false
      }
    })()
  }, [apiRef])

  return { renderMissing }
}

/** Цвет запечён в fileId, но сам SVG красим по элементу — берём его цвет обводки. */
function colorOf(api: ExcalidrawImperativeAPI, fileId: string): string {
  const el = api.getSceneElements().find((e) => e.type === 'image' && e.fileId === fileId)
  return (el?.customData?.color as string) ?? '#1e1e1e'
}
