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
 * проверить тестом: остальное — вызовы Excalidraw. `haveFile` — источник
 * истины про то, что уже отрендерено (см. useMathFiles: он переживает
 * пересоздание инстанса Excalidraw, в отличие от любого локального
 * накопителя). `failed` — формулы, чей рендер уже провалился и которые не
 * стоит пробовать на каждый onChange.
 */
export function formulasNeedingRender(
  elements: readonly ExcalidrawElement[],
  haveFile: ReadonlySet<string>,
  failed: ReadonlySet<string>
): { fileId: string; latex: string }[] {
  const out: { fileId: string; latex: string }[] = []
  const seen = new Set<string>()
  for (const el of elements) {
    if (el.type !== 'image' || el.isDeleted || !el.fileId) continue
    if (haveFile.has(el.fileId) || failed.has(el.fileId) || seen.has(el.fileId)) continue
    const formula = readFormula(el)
    if (!formula) continue
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
 * снова. Признак «уже отрендерено» поэтому берём из api.getFiles() — он живёт
 * в самом инстансе Excalidraw и переживает его пересоздание по key={pageId}
 * (см. hydrateFiles в useExcalidrawSync.ts — тот же приём). Локальный Set
 * держим только для формул, чей рендер уже провалился: в getFiles() их не
 * будет никогда, и без failedRef они бы рендерились заново на каждый кадр.
 */
export function useMathFiles(apiRef: RefObject<ExcalidrawImperativeAPI | null>) {
  const failedRef = useRef(new Set<string>())
  const inFlightRef = useRef(false)

  const renderMissing = useCallback(() => {
    const api = apiRef.current
    if (!api || inFlightRef.current) return

    const haveFile = new Set(Object.keys(api.getFiles()))
    const pending = formulasNeedingRender(api.getSceneElements(), haveFile, failedRef.current)
    if (pending.length === 0) return

    inFlightRef.current = true
    void (async () => {
      try {
        // Модуль тяжёлый (~490 КБ gzip) — грузим при первой формуле на странице.
        const { latexToSvg, svgToDataUrl } = await import('@/lib/latexToSvg')
        const files: BinaryFileData[] = []

        for (const { fileId, latex } of pending) {
          try {
            const { svg, error } = await latexToSvg(latex, { color: colorOf(api, fileId) })
            // Битый SVG не отдаём: Excalidraw пометил бы элемент status:'error'
            // через newElementWith, а это version++ — порча уехала бы всем пирам.
            if (error || !svg) {
              failedRef.current.add(fileId)
              continue
            }
            files.push({
              id: fileId as FileId,
              dataURL: svgToDataUrl(svg) as DataURL,
              mimeType: 'image/svg+xml',
              created: Date.now(),
            })
          } catch (err) {
            // try/catch на КАЖДЫЙ элемент, а не вокруг всего цикла: обвал
            // рендера одной формулы (патологический LaTeX, срыв загрузки
            // чанка шрифта) не должен унести с собой уже отрендеренные
            // элементы 1..N-1 — они и так ещё не дошли до addFiles. В
            // failedRef НЕ заносим: это может быть транзиентный сбой (сеть,
            // сорвавшийся чанк шрифта), а не свойство самого LaTeX — в
            // отличие от ветки error выше, блокировать формулу навсегда
            // (до перезагрузки вкладки) тут неверно, следующий onChange
            // должен получить шанс попробовать снова.
            console.warn('Формулу не удалось отрендерить', err)
          }
        }

        if (files.length > 0) apiRef.current?.addFiles(files)
      } catch (err) {
        // Сюда попадает только сбой самого динамического импорта — отдельные
        // формулы в failedRef не попадают, следующий onChange повторит батч.
        console.warn('Не удалось загрузить рендерер формул', err)
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
  // customData — произвольный JSON чужого элемента Excalidraw, каст на совести
  // вызывающего кода; полю просто нет типа в апстриме.
  return (el?.customData?.color as string) ?? '#1e1e1e'
}
