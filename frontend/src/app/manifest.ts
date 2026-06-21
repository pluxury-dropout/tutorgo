import type { MetadataRoute } from 'next'

// ponytail: минимальный manifest ради display:standalone (Android Chrome открывает PWA без браузерного UI).
// Иконка — favicon.ico как фолбэк; добавить 192/512 PNG, когда появится настоящий логотип (нужно для install-промпта).
export default function manifest(): MetadataRoute.Manifest {
  return {
    name: 'TutorGo',
    short_name: 'TutorGo',
    description: 'CRM for private tutors',
    start_url: '/',
    display: 'standalone',
    background_color: '#ffffff',
    theme_color: '#337EA9',
    icons: [{ src: '/favicon.ico', sizes: 'any', type: 'image/x-icon' }],
  }
}
