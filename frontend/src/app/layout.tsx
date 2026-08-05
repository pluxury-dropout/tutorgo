import type { Metadata, Viewport } from 'next'
import { Plus_Jakarta_Sans } from 'next/font/google'
import './globals.css'
import { Providers } from './providers'

const plusJakartaSans = Plus_Jakarta_Sans({
  variable: '--font-sans',
  subsets: ['latin'],
  weight: ['400', '500', '600', '700'],
})

export const viewport: Viewport = {
  width: 'device-width',
  initialScale: 1,
  maximumScale: 1,
  userScalable: false,
  viewportFit: 'cover',
  colorScheme: 'light dark', // нативные контролы/скроллбары следуют теме
  themeColor: [
    { media: '(prefers-color-scheme: light)', color: '#F8F8F9' }, // = --background
    { media: '(prefers-color-scheme: dark)', color: '#141414' },
  ],
}

export const metadata: Metadata = {
  title: 'Amida',
  description: 'Платформа для преподавателей',
  // ponytail: только apple-теги — iOS не читает manifest display:standalone. Manifest добавить, когда понадобится Android PWA.
  appleWebApp: {
    capable: true,
    statusBarStyle: 'default',
    title: 'Amida',
  },
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="ru" className={`${plusJakartaSans.variable} h-full antialiased`} suppressHydrationWarning>
      <body className="min-h-full bg-background text-foreground">
        <Providers>{children}</Providers>
      </body>
    </html>
  )
}
