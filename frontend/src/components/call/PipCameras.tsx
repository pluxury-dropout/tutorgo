'use client'

import { ParticipantTile, useTracks } from '@livekit/components-react'
import { Track } from 'livekit-client'

export function PipCameras() {
  const tracks = useTracks(
    [{ source: Track.Source.Camera, withPlaceholder: true }],
    { onlySubscribed: false },
  )

  // ponytail: withPlaceholder может на миг отдать placeholder + реальный трек
  // для одного participant+source (гонка при перепубликации камеры) → дубль ключа.
  // Дедупим, предпочитая реальный трек (с publication) заглушке.
  const byKey = new Map<string, (typeof tracks)[number]>()
  for (const t of tracks) {
    const key = `${t.participant.identity}-${t.source}`
    const existing = byKey.get(key)
    if (!existing || ('publication' in t && t.publication)) byKey.set(key, t)
  }
  const uniqueTracks = [...byKey.values()]

  return (
    <div
      style={{
        position: 'fixed',
        top: 12,
        right: 12,
        zIndex: 9999,
        display: 'flex',
        flexDirection: 'column',
        gap: 8,
        pointerEvents: 'auto',
      }}
    >
      {uniqueTracks.map((track) => (
        <ParticipantTile
          key={`${track.participant.identity}-${track.source}`}
          trackRef={track}
          style={{
            width: 120,
            height: 80,
            borderRadius: 8,
            overflow: 'hidden',
            flexShrink: 0,
          }}
        />
      ))}
    </div>
  )
}
