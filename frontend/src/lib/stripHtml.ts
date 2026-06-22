// HTML → простой однострочный текст. Для отображения форматированного title
// задачи там, где разметка не поддерживается (события FullCalendar).
export function stripHtml(html: string): string {
  if (!html) return ''
  // На сервере DOMParser нет — грубый regex-фолбэк (на клиенте парсим корректно).
  if (typeof window === 'undefined') {
    return html.replace(/<[^>]*>/g, ' ').replace(/\s+/g, ' ').trim()
  }
  const text = new DOMParser().parseFromString(html, 'text/html').body.textContent ?? ''
  return text.replace(/\s+/g, ' ').trim()
}
