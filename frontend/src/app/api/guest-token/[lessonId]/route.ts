import { NextRequest, NextResponse } from 'next/server'

export async function GET(
  _req: NextRequest,
  { params }: { params: Promise<{ lessonId: string }> },
) {
  const { lessonId } = await params
  const backendURL = process.env.API_URL ?? 'http://localhost:8080'

  const res = await fetch(`${backendURL}/public/lessons/${lessonId}/guest-token`, {
    cache: 'no-store',
  })

  const data = await res.json()
  return NextResponse.json(data, { status: res.status })
}
