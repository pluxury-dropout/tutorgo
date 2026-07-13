import * as pdfjs from 'pdfjs-dist'
import type { PDFDocumentProxy } from 'pdfjs-dist'

pdfjs.GlobalWorkerOptions.workerSrc = new URL(
  'pdfjs-dist/build/pdf.worker.min.mjs',
  import.meta.url
).toString()

export type RenderedPage = { blob: Blob; width: number; height: number }

export async function loadPdf(file: File): Promise<PDFDocumentProxy> {
  const arrayBuffer = await file.arrayBuffer()
  return pdfjs.getDocument({ data: arrayBuffer }).promise
}

// Рендерит одну страницу (1-индексированную) в PNG.
export async function renderPage(
  pdf: PDFDocumentProxy,
  pageNum: number
): Promise<RenderedPage> {
  const page = await pdf.getPage(pageNum)
  const viewport = page.getViewport({ scale: 1.5 })
  const canvas = document.createElement('canvas')
  canvas.width = viewport.width
  canvas.height = viewport.height
  await page.render({ canvas, viewport }).promise
  const blob = await new Promise<Blob>((res) =>
    canvas.toBlob((b) => res(b!), 'image/png')
  )
  return { blob, width: viewport.width, height: viewport.height }
}
