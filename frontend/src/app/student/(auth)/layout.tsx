export default function StudentAuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-[100dvh] flex items-center justify-center bg-muted/40 px-4">
      <div className="w-full max-w-md">{children}</div>
    </div>
  )
}
