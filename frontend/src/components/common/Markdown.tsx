import ReactMarkdown from 'react-markdown'
import rehypeSanitize from 'rehype-sanitize'

// Рендер ДЗ, написанного репетитором. rehype-sanitize — обязательно: текст
// авторский, но показывается ученику, поэтому вырезаем любой опасный HTML.
export function Markdown({ children }: { children: string }) {
  return (
    <div className="prose-homework" style={{ fontSize: 14, lineHeight: 1.5 }}>
      <ReactMarkdown rehypePlugins={[rehypeSanitize]}>{children}</ReactMarkdown>
    </div>
  )
}
