// HTML → простой однострочный текст. Для отображения форматированного title
// задачи там, где разметка не поддерживается (события FullCalendar).
export function stripHtml(html: string): string {
  if (!html) return ''
  // DOMParser доступен в браузере; компонент-вызыватель — клиентский.
  const text = new DOMParser().parseFromString(html, 'text/html').body.textContent ?? ''
  return text.replace(/\s+/g, ' ').trim()
}
