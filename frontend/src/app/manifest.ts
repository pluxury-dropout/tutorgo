import type { MetadataRoute } from 'next'

// ponytail: минимальный manifest ради display:standalone (Android Chrome открывает PWA без браузерного UI).
// Иконка — favicon.ico как фолбэк; добавить 192/512 PNG, когда появится настоящий логотип (нужно для install-промпта).
export default function manifest(): MetadataRoute.Manifest {
  return {
    name: 'TutorHub',
    short_name: 'TutorHub',
    description: 'CRM for private tutors',
    start_url: '/',
    display: 'standalone',
    background_color: '#F8F8F9',
    theme_color: '#222428',
    icons: [{ src: '/favicon.ico', sizes: 'any', type: 'image/x-icon' }],
  }
}
