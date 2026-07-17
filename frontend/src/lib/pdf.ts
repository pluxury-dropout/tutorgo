import * as pdfjs from 'pdfjs-dist'
import type { PDFDocumentProxy } from 'pdfjs-dist'

pdfjs.GlobalWorkerOptions.workerSrc = new URL(
  'pdfjs-dist/build/pdf.worker.min.mjs',
  import.meta.url
).toString()

export type RenderedPage = {
  blob: Blob
  mimeType: string
  width: number
  height: number
}

export async function loadPdf(file: File): Promise<PDFDocumentProxy> {
  const arrayBuffer = await file.arrayBuffer()
  return pdfjs.getDocument({ data: arrayBuffer }).promise
}

// Размер страницы на доске (единицы сцены) и плотность растра.
// RENDER_SCALE > LAYOUT_SCALE — запас пикселей, чтобы при зуме не мылило.
const LAYOUT_SCALE = 1.5
const RENDER_SCALE = 4 // ponytail: константа, не devicePixelRatio — доску зумят сильнее экрана

// WebP вместо PNG: тот же растр примерно в 8 раз легче, а заливка в S3 — главная
// цена вставки PDF. Качество 0.85 на скане/тексте визуально неотличимо.
// toBlob по спеке молча отдаёт PNG, если тип не поддержан, поэтому реальный тип
// читаем из blob.type, а не предполагаем.
const RASTER_TYPE = 'image/webp'
const RASTER_QUALITY = 0.85

// Габариты страницы в единицах доски, без растеризации — дёшево. Нужны, чтобы
// разложить и показать все страницы плейсхолдерами до того, как отрисуется первая.
export async function pageSize(
  pdf: PDFDocumentProxy,
  pageNum: number
): Promise<{ w: number; h: number }> {
  const page = await pdf.getPage(pageNum)
  const vp = page.getViewport({ scale: LAYOUT_SCALE })
  return { w: vp.width, h: vp.height }
}

// Рендерит одну страницу (1-индексированную) в растр.
export async function renderPage(
  pdf: PDFDocumentProxy,
  pageNum: number
): Promise<RenderedPage> {
  const page = await pdf.getPage(pageNum)
  const viewport = page.getViewport({ scale: RENDER_SCALE })
  const canvas = document.createElement('canvas')
  canvas.width = viewport.width
  canvas.height = viewport.height
  await page.render({ canvas, viewport }).promise
  const blob = await new Promise<Blob>((res) =>
    canvas.toBlob((b) => res(b!), RASTER_TYPE, RASTER_QUALITY)
  )
  // Растр плотнее, чем место на доске → Excalidraw есть что показать при зуме.
  const k = LAYOUT_SCALE / RENDER_SCALE
  return {
    blob,
    mimeType: blob.type || 'image/png',
    width: viewport.width * k,
    height: viewport.height * k,
  }
}
