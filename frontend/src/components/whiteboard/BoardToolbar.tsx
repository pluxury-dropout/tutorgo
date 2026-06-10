'use client'

import { useState, useRef } from 'react'
import * as pdfjs from 'pdfjs-dist'
import { useCreateInvite } from '@/lib/hooks/useWhiteboard'
import { whiteboardApi, BASE_URL } from '@/lib/api/whiteboard'

// Point to the PDF.js worker
pdfjs.GlobalWorkerOptions.workerSrc = `//cdnjs.cloudflare.com/ajax/libs/pdf.js/${pdfjs.version}/pdf.worker.min.js`

interface Props {
  boardId: string
  isGuest?: boolean
}

export function BoardToolbar({ boardId, isGuest = false }: Props) {
  const [copying, setCopying] = useState(false)
  const [uploading, setUploading] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)
  const createInvite = useCreateInvite(boardId)

  const handleCopyInvite = async () => {
    const inv = await createInvite.mutateAsync()
    const url = `${window.location.origin}/board/join/${inv.id}`
    await navigator.clipboard.writeText(url)
    setCopying(true)
    setTimeout(() => setCopying(false), 2000)
  }

  const handlePdfUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
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
    } finally {
      setUploading(false)
      if (fileRef.current) fileRef.current.value = ''
    }
  }

  return (
    <div className="flex items-center gap-2 px-3 py-2 border-b bg-white">
      {!isGuest && (
        <button
          onClick={handleCopyInvite}
          className="text-sm px-3 py-1 rounded bg-blue-50 hover:bg-blue-100 text-blue-700 border border-blue-200"
        >
          {copying ? 'Скопировано!' : 'Пригласить ученика'}
        </button>
      )}
      <button
        onClick={() => fileRef.current?.click()}
        disabled={uploading}
        className="text-sm px-3 py-1 rounded bg-gray-50 hover:bg-gray-100 text-gray-700 border border-gray-200"
      >
        {uploading ? 'Загрузка...' : 'PDF / Изображение'}
      </button>
      <input
        ref={fileRef}
        type="file"
        accept="image/*,application/pdf"
        className="hidden"
        onChange={handlePdfUpload}
      />
    </div>
  )
}
