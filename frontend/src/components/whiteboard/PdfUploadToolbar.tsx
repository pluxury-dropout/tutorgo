'use client'

import { useRef, useState } from 'react'
import { toast } from 'sonner'
import * as pdfjs from 'pdfjs-dist'
import { DefaultToolbar, DefaultToolbarContent } from '@tldraw/tldraw'
import { whiteboardApi, BASE_URL } from '@/lib/api/whiteboard'
import { useBoardContext } from './BoardContext'

pdfjs.GlobalWorkerOptions.workerSrc = new URL(
  'pdfjs-dist/build/pdf.worker.min.mjs',
  import.meta.url
).toString()

function PdfUploadButton({ boardId }: { boardId: string }) {
  const [uploading, setUploading] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)
  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setUploading(true)
    try {
      if (file.type === 'application/pdf') {
        const arrayBuffer = await file.arrayBuffer()
        const pdf = await pdfjs.getDocument({ data: arrayBuffer }).promise
        for (let i = 1; i <= pdf.numPages; i++) {
          const page = await pdf.getPage(i)
          const viewport = page.getViewport({ scale: 1.5 })
          const canvas = document.createElement('canvas')
          canvas.width = viewport.width
          canvas.height = viewport.height
          await page.render({ canvas, viewport }).promise
          const blob = await new Promise<Blob>((res) =>
            canvas.toBlob((b) => res(b!), 'image/png')
          )
          const pngFile = new File([blob], `page-${i}.png`, { type: 'image/png' })
          const result = await whiteboardApi.uploadAsset(boardId, pngFile)
          window.dispatchEvent(
            new CustomEvent('wb:insert-image', {
              detail: {
                url: `${BASE_URL}${result.url}`,
                width: viewport.width,
                height: viewport.height,
              },
            })
          )
        }
      } else {
        const result = await whiteboardApi.uploadAsset(boardId, file)
        window.dispatchEvent(
          new CustomEvent('wb:insert-image', {
            detail: { url: `${BASE_URL}${result.url}` },
          })
        )
      }
    } catch {
      toast.error('Не удалось загрузить файл')
    } finally {
      setUploading(false)
      if (fileRef.current) fileRef.current.value = ''
    }
  }

  return (
    <>
      <button
        onClick={() => fileRef.current?.click()}
        disabled={uploading}
        className="tlui-button tlui-button__normal"
        title="Загрузить PDF или изображение"
      >
        {uploading ? '...' : 'PDF / Фото'}
      </button>
      <input
        ref={fileRef}
        type="file"
        accept="image/*,application/pdf"
        className="hidden"
        onChange={handleUpload}
      />
    </>
  )
}

export function PdfUploadToolbar() {
  const { boardId, isGuest } = useBoardContext()

  return (
    <DefaultToolbar>
      {!isGuest && <PdfUploadButton boardId={boardId} />}
      <DefaultToolbarContent />
    </DefaultToolbar>
  )
}
