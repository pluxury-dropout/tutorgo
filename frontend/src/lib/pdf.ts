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

// Рендерит страницы [from, to] (включительно, 1-индексированные) в PNG.
export async function renderPages(
  pdf: PDFDocumentProxy,
  from: number,
  to: number,
  onProgress?: (done: number, total: number) => void
): Promise<RenderedPage[]> {
  const total = to - from + 1
  const pages: RenderedPage[] = []
  for (let i = from; i <= to; i++) {
    const page = await pdf.getPage(i)
    const viewport = page.getViewport({ scale: 1.5 })
    const canvas = document.createElement('canvas')
    canvas.width = viewport.width
    canvas.height = viewport.height
    await page.render({ canvas, viewport }).promise
    const blob = await new Promise<Blob>((res) =>
      canvas.toBlob((b) => res(b!), 'image/png')
    )
    pages.push({ blob, width: viewport.width, height: viewport.height })
    onProgress?.(pages.length, total)
  }
  return pages
}
