import { NextRequest, NextResponse } from 'next/server'

export async function GET(
  req: NextRequest,
  { params }: { params: Promise<{ roomId: string }> },
) {
  const { roomId } = await params
  const name = req.nextUrl.searchParams.get('name') ?? ''
  const backendURL = process.env.NEXT_PUBLIC_API_URL?.replace(/\/$/, '')

  if (!backendURL) {
    return NextResponse.json({ error: 'NEXT_PUBLIC_API_URL not configured' }, { status: 503 })
  }

  try {
    const url = `${backendURL}/public/quick/${roomId}/guest-token?name=${encodeURIComponent(name)}`
    const res = await fetch(url, { cache: 'no-store' })
    const data = await res.json()
    return NextResponse.json(data, { status: res.status })
  } catch {
    return NextResponse.json({ error: 'backend unavailable' }, { status: 502 })
  }
}
