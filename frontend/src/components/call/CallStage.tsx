// frontend/src/components/call/CallStage.tsx
'use client'

import { useTracks } from '@livekit/components-react'
import { Track } from 'livekit-client'
import { layoutForCount, STAGE_BG } from './callTheme'
import { CallTile } from './CallTile'

export function CallStage() {
  const tracks = useTracks(
    [
      { source: Track.Source.Camera, withPlaceholder: true },
      { source: Track.Source.ScreenShare, withPlaceholder: false },
    ],
    { onlySubscribed: false },
  )

  // Дедуп placeholder+реальный трек одного participant+source (гонка
  // перепубликации камеры), предпочитая реальный трек — как в PipCameras.
  const byKey = new Map<string, (typeof tracks)[number]>()
  for (const t of tracks) {
    const key = `${t.participant.identity}-${t.source}`
    const existing = byKey.get(key)
    if (!existing || ('publication' in t && t.publication)) byKey.set(key, t)
  }
  const unique = [...byKey.values()]
  const layout = layoutForCount(unique.length)

  return (
    <div style={{ width: '100%', height: '100%', background: STAGE_BG }}>
      {layout.mode === 'single' ? (
        <div style={{ width: '100%', height: '100%' }}>
          {unique[0] && <CallTile trackRef={unique[0]} variant="speaker" />}
        </div>
      ) : (
        <div
          style={{
            width: '100%',
            height: '100%',
            display: 'grid',
            gridTemplateColumns: `repeat(${layout.columns}, 1fr)`,
            gap: 10,
            padding: 18,
            boxSizing: 'border-box',
          }}
        >
          {unique.map((t) => (
            <CallTile key={`${t.participant.identity}-${t.source}`} trackRef={t} variant="grid" />
          ))}
        </div>
      )}
    </div>
  )
}
