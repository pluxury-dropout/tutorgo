import { NextRequest, NextResponse } from 'next/server'

export async function GET(
  _req: NextRequest,
  { params }: { params: Promise<{ lessonId: string }> },
) {
  const { lessonId } = await params
  const backendURL = process.env.NEXT_PUBLIC_API_URL?.replace(/\/$/, '')

  if (!backendURL) {
    return NextResponse.json({ error: 'NEXT_PUBLIC_API_URL not configured' }, { status: 503 })
  }

  try {
    const res = await fetch(`${backendURL}/public/lessons/${lessonId}/guest-token`, {
      cache: 'no-store',
    })
    const data = await res.json()
    return NextResponse.json(data, { status: res.status })
  } catch {
    return NextResponse.json({ error: 'backend unavailable' }, { status: 502 })
  }
}
