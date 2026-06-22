'use client'

import { useEditor, EditorContent } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'

// Инлайн-редактор задачи. StarterKit включает markdown-инпут-правила:
// `- `/`* ` → буллет, `1. ` → нумерованный список, `# ` → заголовок,
// `**текст**` → жирный, `> ` → цитата, `` ` `` → код.
export default function TaskEditor({
  defaultValue,
  onCommit,
  onCancel,
}: {
  defaultValue: string
  onCommit: (html: string) => void
  onCancel: () => void
}) {
  const editor = useEditor({
    extensions: [StarterKit],
    content: defaultValue || '',
    autofocus: 'end',
    immediatelyRender: false,
    editorProps: {
      attributes: { class: 'task-content task-editor', 'data-placeholder': 'Название задачи' },
      handleKeyDown: (_view, event) => {
        if (event.key === 'Escape') { onCancel(); return true }
        if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
          commit(); return true
        }
        return false
      },
    },
  })

  function commit() {
    if (!editor) return
    const html = editor.getText().trim() === '' ? '' : editor.getHTML()
    onCommit(html)
  }

  if (!editor) return null

  return (
    <div
      style={{
        borderRadius: 6,
        padding: '8px 10px',
        marginBottom: 6,
        background: 'var(--card)',
        border: '1px solid var(--border)',
        fontSize: 13,
      }}
      onBlur={(e) => {
        // Коммитим только когда фокус ушёл из всего редактора, а не между его узлами.
        if (!e.currentTarget.contains(e.relatedTarget as Node)) commit()
      }}
    >
      <EditorContent editor={editor} />
    </div>
  )
}
